package controller

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/songquanpeng/one-api/common/config"
	"github.com/songquanpeng/one-api/common/ctxkey"
	"github.com/songquanpeng/one-api/model"
)

type CreatePaymentOrderRequest struct {
	PackageId int    `json:"package_id"`
	Provider  string `json:"provider"`
}

type RefundPaymentOrderRequest struct {
	Reason string `json:"reason"`
}

func ListAvailableTopUpPackages(c *gin.Context) {
	packages, err := model.GetAvailableTopUpPackages()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": packages})
}

func GetAllTopUpPackages(c *gin.Context) {
	p, _ := strconv.Atoi(c.Query("p"))
	if p < 0 {
		p = 0
	}
	packages, err := model.GetAllTopUpPackages(p*config.ItemsPerPage, config.ItemsPerPage)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": packages})
}

func GetTopUpPackage(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	p, err := model.GetTopUpPackageById(id)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": p})
}

func AddTopUpPackage(c *gin.Context) {
	p := model.TopUpPackage{}
	if err := c.ShouldBindJSON(&p); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	if err := p.Insert(); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": p})
}

func UpdateTopUpPackage(c *gin.Context) {
	p := model.TopUpPackage{}
	if err := c.ShouldBindJSON(&p); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	if p.Id == 0 {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "id 为空"})
		return
	}
	if err := p.Update(); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": p})
}

func DeleteTopUpPackage(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := model.DeleteTopUpPackageById(id); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": ""})
}

func CreatePaymentOrder(c *gin.Context) {
	req := CreatePaymentOrderRequest{}
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	if err := EnsurePaymentProviderConfigured(req.Provider); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	order, err := model.CreatePaymentOrder(c.GetInt(ctxkey.Id), req.PackageId, req.Provider)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	paymentUrl, err := BuildPaymentUrl(c, order)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error(), "data": order})
		return
	}
	order.PaymentUrl = paymentUrl
	_ = model.UpdatePaymentOrderUrl(order.Id, paymentUrl)
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": order})
}

func GetPaymentOrder(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	order, err := model.GetPaymentOrderById(id, c.GetInt(ctxkey.Id), false)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": order})
}

func GetUserPaymentOrders(c *gin.Context) {
	p, _ := strconv.Atoi(c.Query("p"))
	if p < 0 {
		p = 0
	}
	orders, err := model.GetUserPaymentOrders(c.GetInt(ctxkey.Id), p*config.ItemsPerPage, config.ItemsPerPage)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": orders})
}

func ClosePaymentOrder(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := model.ClosePaymentOrder(id, c.GetInt(ctxkey.Id), false); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": ""})
}

func CompleteMockPaymentOrder(c *gin.Context) {
	if err := EnsurePaymentProviderConfigured("mock"); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	id, _ := strconv.Atoi(c.Param("id"))
	order, err := model.GetPaymentOrderById(id, c.GetInt(ctxkey.Id), false)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	if order.Provider != "mock" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "该订单不是模拟支付订单"})
		return
	}
	paidOrder, err := model.CompletePaymentOrder(c.Request.Context(), order.OrderNo, "mock", "mock-"+order.OrderNo)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	_ = model.RecordPaymentEvent("mock", order.OrderNo, "mock-"+order.OrderNo, "mock_complete", "{}", model.PaymentEventStatusProcessed)
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": paidOrder})
}

func GetAllPaymentOrders(c *gin.Context) {
	p, _ := strconv.Atoi(c.Query("p"))
	if p < 0 {
		p = 0
	}
	status, _ := strconv.Atoi(c.Query("status"))
	userId, _ := strconv.Atoi(c.Query("user_id"))
	filter := model.PaymentOrderFilter{
		Keyword:  strings.TrimSpace(c.Query("keyword")),
		Provider: strings.TrimSpace(c.Query("provider")),
		Status:   status,
		UserId:   userId,
	}
	orders, err := model.GetAllPaymentOrders(filter, p*config.ItemsPerPage, config.ItemsPerPage)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": orders})
}

func AdminClosePaymentOrder(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := model.ClosePaymentOrder(id, 0, true); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": ""})
}

func AdminMarkPaymentOrderPaid(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	order, err := model.GetPaymentOrderById(id, 0, true)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	paidOrder, err := model.CompletePaymentOrder(c.Request.Context(), order.OrderNo, order.Provider, "admin-manual-"+order.OrderNo)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": paidOrder})
}

func AdminRefundPaymentOrder(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	req := RefundPaymentOrderRequest{}
	_ = c.ShouldBindJSON(&req)
	refundedOrder, err := model.RefundPaymentOrder(c.Request.Context(), id, strings.TrimSpace(req.Reason))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	_ = model.RecordPaymentEvent(refundedOrder.Provider, refundedOrder.OrderNo, refundedOrder.ProviderTradeNo, "admin_refund", req.Reason, model.PaymentEventStatusProcessed)
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": refundedOrder})
}

func GetPaymentConfigStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"wechat": GetPaymentProviderStatus("wechat"),
			"alipay": GetPaymentProviderStatus("alipay"),
			"mock":   GetPaymentProviderStatus("mock"),
		},
	})
}

func GetUserPaymentConfigStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"wechat": gin.H{"enabled": GetPaymentProviderStatus("wechat")["enabled"]},
			"alipay": gin.H{"enabled": GetPaymentProviderStatus("alipay")["enabled"]},
			"mock":   gin.H{"enabled": GetPaymentProviderStatus("mock")["enabled"]},
		},
	})
}

func GetUserQuotaGrants(c *gin.Context) {
	grants, err := model.GetUserQuotaGrants(c.GetInt(ctxkey.Id))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": grants})
}
