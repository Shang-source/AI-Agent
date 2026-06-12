# 支付联调说明

One API 商业化充值支持支付宝电脑网站支付和微信支付 Native v3。

## 本地模拟支付

没有真实商户参数时，可以只在本地开启模拟支付：

```env
MOCK_PAYMENT_ENABLED=true
```

开启后，用户充值页会显示“模拟支付”按钮。点击后会创建 `mock` 订单，再点击“完成模拟支付”即可模拟支付成功、订单变为已支付、额度自动到账。

不要在生产环境开启 `MOCK_PAYMENT_ENABLED=true`。

## 回调地址

生产环境必须配置可公网访问的 HTTPS 域名，并设置：

```env
PAYMENT_NOTIFY_BASE_URL=https://api.example.com
```

支付平台会回调：

- 支付宝：`https://api.example.com/api/payment/alipay/notify`
- 微信支付：`https://api.example.com/api/payment/wechat/notify`

## 支付宝

需要配置：

```env
ALIPAY_APP_ID=
ALIPAY_PRIVATE_KEY=
ALIPAY_PUBLIC_KEY=
ALIPAY_GATEWAY=
```

`ALIPAY_GATEWAY` 可为空，默认使用正式网关 `https://openapi.alipay.com/gateway.do`。沙箱联调时改为支付宝沙箱网关。

## 微信支付

需要配置：

```env
WECHAT_PAY_APP_ID=
WECHAT_PAY_MCH_ID=
WECHAT_PAY_SERIAL_NO=
WECHAT_PAY_PRIVATE_KEY=
WECHAT_PAY_API_V3_KEY=
WECHAT_PAY_PLATFORM_PUBLIC_KEY=
```

微信 Native 支付会返回 `code_url`，前端充值页会把它渲染成二维码。用户支付完成后，页面每 3 秒轮询订单状态；支付回调验签和解密成功后，系统会自动发放额度。

## 联调步骤

1. 管理员在 `/package` 创建并上架一个低价测试套餐。
2. 配置支付环境变量并重启服务。
3. 用户在 `/topup` 选择支付宝或微信购买。
4. 支付完成后检查订单是否变为已支付。
5. 检查用户额度、额度包、充值日志是否同步更新。

不要在未完成验签测试前开放真实用户支付。
