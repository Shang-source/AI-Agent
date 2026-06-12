package controller

import (
	"bytes"
	"crypto"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/songquanpeng/one-api/common/config"
	"github.com/songquanpeng/one-api/common/random"
	"github.com/songquanpeng/one-api/model"
)

const (
	paymentProviderWechat = "wechat"
	paymentProviderAlipay = "alipay"
	paymentProviderMock   = "mock"
)

func paymentEnv(key string) string {
	return strings.TrimSpace(os.Getenv(key))
}

func paymentNotifyBaseURL() string {
	if base := paymentEnv("PAYMENT_NOTIFY_BASE_URL"); base != "" {
		return strings.TrimRight(base, "/")
	}
	return strings.TrimRight(config.ServerAddress, "/")
}

func GetPaymentProviderStatus(provider string) gin.H {
	if provider == paymentProviderMock {
		enabled := strings.EqualFold(paymentEnv("MOCK_PAYMENT_ENABLED"), "true")
		missing := make([]string, 0)
		if !enabled {
			missing = append(missing, "MOCK_PAYMENT_ENABLED=true")
		}
		return gin.H{
			"enabled": enabled,
			"missing": missing,
		}
	}
	keys := paymentRequiredKeys(provider)
	missing := make([]string, 0)
	for _, key := range keys {
		if paymentEnv(key) == "" {
			missing = append(missing, key)
		}
	}
	return gin.H{
		"enabled": len(missing) == 0,
		"missing": missing,
	}
}

func paymentRequiredKeys(provider string) []string {
	switch provider {
	case paymentProviderWechat:
		return []string{
			"WECHAT_PAY_APP_ID",
			"WECHAT_PAY_MCH_ID",
			"WECHAT_PAY_SERIAL_NO",
			"WECHAT_PAY_PRIVATE_KEY",
			"WECHAT_PAY_API_V3_KEY",
			"WECHAT_PAY_PLATFORM_PUBLIC_KEY",
		}
	case paymentProviderAlipay:
		return []string{
			"ALIPAY_APP_ID",
			"ALIPAY_PRIVATE_KEY",
			"ALIPAY_PUBLIC_KEY",
		}
	case paymentProviderMock:
		return []string{"MOCK_PAYMENT_ENABLED"}
	default:
		return []string{"UNKNOWN_PAYMENT_PROVIDER"}
	}
}

func EnsurePaymentProviderConfigured(provider string) error {
	status := GetPaymentProviderStatus(provider)
	if enabled, ok := status["enabled"].(bool); ok && enabled {
		return nil
	}
	return fmt.Errorf("%s 支付配置未完成，请先配置缺失项：%v", provider, status["missing"])
}

func BuildPaymentUrl(c *gin.Context, order *model.PaymentOrder) (string, error) {
	switch order.Provider {
	case paymentProviderWechat:
		return buildWechatNativePaymentUrl(order)
	case paymentProviderAlipay:
		return buildAlipayPagePaymentUrl(order)
	case paymentProviderMock:
		return paymentNotifyBaseURL() + "/topup?mock_order_id=" + strconv.Itoa(order.Id), nil
	default:
		return "", errors.New("不支持的支付方式")
	}
}

func centsToYuan(cents int64) string {
	return fmt.Sprintf("%d.%02d", cents/100, cents%100)
}

func yuanToCents(amount string) (int64, error) {
	parts := strings.SplitN(amount, ".", 2)
	yuan, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, err
	}
	cents := int64(0)
	if len(parts) == 2 {
		decimal := parts[1]
		if len(decimal) == 1 {
			decimal += "0"
		}
		if len(decimal) > 2 {
			decimal = decimal[:2]
		}
		cents, err = strconv.ParseInt(decimal, 10, 64)
		if err != nil {
			return 0, err
		}
	}
	return yuan*100 + cents, nil
}

func sortedPaymentQuery(params map[string]string, excludeSignType bool) string {
	keys := make([]string, 0, len(params))
	for key, value := range params {
		if key == "sign" || value == "" || (excludeSignType && key == "sign_type") {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+params[key])
	}
	return strings.Join(parts, "&")
}

func pemBlockFromEnv(raw string, blockType string) string {
	raw = strings.TrimSpace(raw)
	if strings.Contains(raw, "BEGIN ") {
		return raw
	}
	raw = strings.ReplaceAll(raw, " ", "")
	var builder strings.Builder
	builder.WriteString("-----BEGIN " + blockType + "-----\n")
	for len(raw) > 64 {
		builder.WriteString(raw[:64] + "\n")
		raw = raw[64:]
	}
	builder.WriteString(raw + "\n")
	builder.WriteString("-----END " + blockType + "-----")
	return builder.String()
}

func parseRSAPrivateKey(raw string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemBlockFromEnv(raw, "PRIVATE KEY")))
	if block == nil {
		block, _ = pem.Decode([]byte(pemBlockFromEnv(raw, "RSA PRIVATE KEY")))
	}
	if block == nil {
		return nil, errors.New("无法解析 RSA 私钥")
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("私钥不是 RSA 私钥")
	}
	return key, nil
}

func parseRSAPublicKey(raw string) (*rsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(pemBlockFromEnv(raw, "PUBLIC KEY")))
	if block == nil {
		return nil, errors.New("无法解析 RSA 公钥")
	}
	parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		if cert, certErr := x509.ParseCertificate(block.Bytes); certErr == nil {
			if key, ok := cert.PublicKey.(*rsa.PublicKey); ok {
				return key, nil
			}
		}
		return nil, err
	}
	key, ok := parsed.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("公钥不是 RSA 公钥")
	}
	return key, nil
}

func rsaSHA256Sign(raw string, privateKey string) (string, error) {
	key, err := parseRSAPrivateKey(privateKey)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256([]byte(raw))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, hash[:])
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(signature), nil
}

func rsaSHA256Verify(raw string, signature string, publicKey string) error {
	key, err := parseRSAPublicKey(publicKey)
	if err != nil {
		return err
	}
	signatureBytes, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		return err
	}
	hash := sha256.Sum256([]byte(raw))
	return rsa.VerifyPKCS1v15(key, crypto.SHA256, hash[:], signatureBytes)
}

func buildAlipayPagePaymentUrl(order *model.PaymentOrder) (string, error) {
	gateway := paymentEnv("ALIPAY_GATEWAY")
	if gateway == "" {
		gateway = "https://openapi.alipay.com/gateway.do"
	}
	bizContent, _ := json.Marshal(gin.H{
		"out_trade_no": order.OrderNo,
		"product_code": "FAST_INSTANT_TRADE_PAY",
		"total_amount": centsToYuan(order.AmountCents),
		"subject":      fmt.Sprintf("AI API 额度套餐 %s", order.OrderNo),
	})
	params := map[string]string{
		"app_id":      paymentEnv("ALIPAY_APP_ID"),
		"method":      "alipay.trade.page.pay",
		"format":      "JSON",
		"charset":     "utf-8",
		"sign_type":   "RSA2",
		"timestamp":   time.Now().Format("2006-01-02 15:04:05"),
		"version":     "1.0",
		"notify_url":  paymentNotifyBaseURL() + "/api/payment/alipay/notify",
		"return_url":  paymentNotifyBaseURL() + "/topup?order_no=" + url.QueryEscape(order.OrderNo),
		"biz_content": string(bizContent),
	}
	sign, err := rsaSHA256Sign(sortedPaymentQuery(params, false), paymentEnv("ALIPAY_PRIVATE_KEY"))
	if err != nil {
		return "", err
	}
	values := url.Values{}
	for key, value := range params {
		values.Set(key, value)
	}
	values.Set("sign", sign)
	return gateway + "?" + values.Encode(), nil
}

func buildWechatNativePaymentUrl(order *model.PaymentOrder) (string, error) {
	body, _ := json.Marshal(gin.H{
		"appid":        paymentEnv("WECHAT_PAY_APP_ID"),
		"mchid":        paymentEnv("WECHAT_PAY_MCH_ID"),
		"description":  fmt.Sprintf("AI API 额度套餐 %s", order.OrderNo),
		"out_trade_no": order.OrderNo,
		"notify_url":   paymentNotifyBaseURL() + "/api/payment/wechat/notify",
		"amount": gin.H{
			"total":    order.AmountCents,
			"currency": "CNY",
		},
	})
	method := "POST"
	canonicalURL := "/v3/pay/transactions/native"
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	nonce := random.GetRandomString(32)
	signatureSource := strings.Join([]string{method, canonicalURL, timestamp, nonce, string(body), ""}, "\n")
	signature, err := rsaSHA256Sign(signatureSource, paymentEnv("WECHAT_PAY_PRIVATE_KEY"))
	if err != nil {
		return "", err
	}
	auth := fmt.Sprintf(
		`WECHATPAY2-SHA256-RSA2048 mchid="%s",nonce_str="%s",signature="%s",timestamp="%s",serial_no="%s"`,
		paymentEnv("WECHAT_PAY_MCH_ID"),
		nonce,
		signature,
		timestamp,
		paymentEnv("WECHAT_PAY_SERIAL_NO"),
	)
	req, err := http.NewRequest(method, "https://api.mch.weixin.qq.com"+canonicalURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", auth)
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("微信支付下单失败：%s", string(respBody))
	}
	var payload struct {
		CodeURL string `json:"code_url"`
	}
	if err := json.Unmarshal(respBody, &payload); err != nil {
		return "", err
	}
	if payload.CodeURL == "" {
		return "", errors.New("微信支付未返回 code_url")
	}
	return payload.CodeURL, nil
}

func AlipayNotify(c *gin.Context) {
	if err := c.Request.ParseForm(); err != nil {
		c.String(http.StatusOK, "failure")
		return
	}
	params := make(map[string]string)
	for key, values := range c.Request.PostForm {
		if len(values) > 0 {
			params[key] = values[0]
		}
	}
	orderNo := params["out_trade_no"]
	tradeNo := params["trade_no"]
	rawPayload, _ := json.Marshal(params)
	if err := rsaSHA256Verify(sortedPaymentQuery(params, true), params["sign"], paymentEnv("ALIPAY_PUBLIC_KEY")); err != nil {
		_ = model.RecordPaymentEvent(paymentProviderAlipay, orderNo, tradeNo, "notify", string(rawPayload), model.PaymentEventStatusRejected)
		c.String(http.StatusOK, "failure")
		return
	}
	if params["trade_status"] != "TRADE_SUCCESS" && params["trade_status"] != "TRADE_FINISHED" {
		_ = model.RecordPaymentEvent(paymentProviderAlipay, orderNo, tradeNo, params["trade_status"], string(rawPayload), model.PaymentEventStatusReceived)
		c.String(http.StatusOK, "success")
		return
	}
	order, err := model.GetPaymentOrderByOrderNo(orderNo)
	if err != nil || order.Provider != paymentProviderAlipay {
		_ = model.RecordPaymentEvent(paymentProviderAlipay, orderNo, tradeNo, "notify", string(rawPayload), model.PaymentEventStatusRejected)
		c.String(http.StatusOK, "failure")
		return
	}
	paidCents, err := yuanToCents(params["total_amount"])
	if err != nil || paidCents != order.AmountCents {
		_ = model.RecordPaymentEvent(paymentProviderAlipay, orderNo, tradeNo, "amount_mismatch", string(rawPayload), model.PaymentEventStatusRejected)
		c.String(http.StatusOK, "failure")
		return
	}
	if _, err := model.CompletePaymentOrder(c.Request.Context(), orderNo, paymentProviderAlipay, tradeNo); err != nil {
		_ = model.RecordPaymentEvent(paymentProviderAlipay, orderNo, tradeNo, "complete_failed", string(rawPayload), model.PaymentEventStatusRejected)
		c.String(http.StatusOK, "failure")
		return
	}
	_ = model.RecordPaymentEvent(paymentProviderAlipay, orderNo, tradeNo, "notify", string(rawPayload), model.PaymentEventStatusProcessed)
	c.String(http.StatusOK, "success")
}

type wechatNotifyResource struct {
	Algorithm      string `json:"algorithm"`
	Ciphertext     string `json:"ciphertext"`
	AssociatedData string `json:"associated_data"`
	Nonce          string `json:"nonce"`
	OriginalType   string `json:"original_type"`
}

func WechatNotify(c *gin.Context) {
	body, _ := io.ReadAll(c.Request.Body)
	orderNo := ""
	transactionId := ""
	if err := verifyWechatNotifySignature(c, string(body)); err != nil {
		_ = model.RecordPaymentEvent(paymentProviderWechat, orderNo, transactionId, "notify", string(body), model.PaymentEventStatusRejected)
		c.JSON(http.StatusUnauthorized, gin.H{"code": "SIGN_ERROR", "message": err.Error()})
		return
	}
	var notify struct {
		EventType string               `json:"event_type"`
		Resource  wechatNotifyResource `json:"resource"`
	}
	if err := json.Unmarshal(body, &notify); err != nil {
		_ = model.RecordPaymentEvent(paymentProviderWechat, orderNo, transactionId, "notify", string(body), model.PaymentEventStatusRejected)
		c.JSON(http.StatusBadRequest, gin.H{"code": "PARAM_ERROR", "message": err.Error()})
		return
	}
	plainText, err := decryptWechatResource(notify.Resource)
	if err != nil {
		_ = model.RecordPaymentEvent(paymentProviderWechat, orderNo, transactionId, notify.EventType, string(body), model.PaymentEventStatusRejected)
		c.JSON(http.StatusBadRequest, gin.H{"code": "DECRYPT_ERROR", "message": err.Error()})
		return
	}
	var transaction struct {
		OutTradeNo    string `json:"out_trade_no"`
		TransactionId string `json:"transaction_id"`
		TradeState    string `json:"trade_state"`
		Amount        struct {
			Total int64 `json:"total"`
		} `json:"amount"`
	}
	if err := json.Unmarshal([]byte(plainText), &transaction); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "PARAM_ERROR", "message": err.Error()})
		return
	}
	orderNo = transaction.OutTradeNo
	transactionId = transaction.TransactionId
	if transaction.TradeState != "SUCCESS" {
		_ = model.RecordPaymentEvent(paymentProviderWechat, orderNo, transactionId, transaction.TradeState, plainText, model.PaymentEventStatusReceived)
		c.JSON(http.StatusOK, gin.H{"code": "SUCCESS", "message": "成功"})
		return
	}
	order, err := model.GetPaymentOrderByOrderNo(orderNo)
	if err != nil || order.Provider != paymentProviderWechat || order.AmountCents != transaction.Amount.Total {
		_ = model.RecordPaymentEvent(paymentProviderWechat, orderNo, transactionId, "amount_or_order_mismatch", plainText, model.PaymentEventStatusRejected)
		c.JSON(http.StatusBadRequest, gin.H{"code": "ORDER_ERROR", "message": "订单校验失败"})
		return
	}
	if _, err := model.CompletePaymentOrder(c.Request.Context(), orderNo, paymentProviderWechat, transactionId); err != nil {
		_ = model.RecordPaymentEvent(paymentProviderWechat, orderNo, transactionId, "complete_failed", plainText, model.PaymentEventStatusRejected)
		c.JSON(http.StatusBadRequest, gin.H{"code": "ORDER_ERROR", "message": err.Error()})
		return
	}
	_ = model.RecordPaymentEvent(paymentProviderWechat, orderNo, transactionId, notify.EventType, plainText, model.PaymentEventStatusProcessed)
	c.JSON(http.StatusOK, gin.H{"code": "SUCCESS", "message": "成功"})
}

func verifyWechatNotifySignature(c *gin.Context, body string) error {
	signature := c.GetHeader("Wechatpay-Signature")
	timestamp := c.GetHeader("Wechatpay-Timestamp")
	nonce := c.GetHeader("Wechatpay-Nonce")
	if signature == "" || timestamp == "" || nonce == "" {
		return errors.New("微信支付回调缺少签名头")
	}
	signatureSource := strings.Join([]string{timestamp, nonce, body, ""}, "\n")
	return rsaSHA256Verify(signatureSource, signature, paymentEnv("WECHAT_PAY_PLATFORM_PUBLIC_KEY"))
}

func decryptWechatResource(resource wechatNotifyResource) (string, error) {
	if resource.Algorithm != "AEAD_AES_256_GCM" {
		return "", errors.New("不支持的微信支付加密算法")
	}
	key := []byte(paymentEnv("WECHAT_PAY_API_V3_KEY"))
	if len(key) != 32 {
		return "", errors.New("WECHAT_PAY_API_V3_KEY 必须为 32 字节")
	}
	ciphertext, err := base64.StdEncoding.DecodeString(resource.Ciphertext)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	plain, err := aead.Open(nil, []byte(resource.Nonce), ciphertext, []byte(resource.AssociatedData))
	if err != nil {
		return "", err
	}
	return string(plain), nil
}
