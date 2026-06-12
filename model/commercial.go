package model

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/songquanpeng/one-api/common"
	"github.com/songquanpeng/one-api/common/helper"
	"github.com/songquanpeng/one-api/common/random"
	"gorm.io/gorm"
)

const (
	TopUpPackageStatusEnabled  = 1
	TopUpPackageStatusDisabled = 2

	TopUpPackageTypeFixed    = "fixed"
	TopUpPackageTypeDuration = "duration"

	PaymentOrderStatusPending  = 1
	PaymentOrderStatusPaid     = 2
	PaymentOrderStatusClosed   = 3
	PaymentOrderStatusFailed   = 4
	PaymentOrderStatusRefunded = 5

	QuotaGrantStatusActive  = 1
	QuotaGrantStatusExpired = 2

	PaymentEventStatusReceived  = 1
	PaymentEventStatusProcessed = 2
	PaymentEventStatusRejected  = 3
)

type TopUpPackage struct {
	Id           int    `json:"id"`
	Name         string `json:"name" gorm:"index"`
	Description  string `json:"description" gorm:"type:text"`
	Type         string `json:"type" gorm:"default:fixed;index"`
	PriceCents   int64  `json:"price_cents" gorm:"bigint;default:0"`
	Quota        int64  `json:"quota" gorm:"bigint;default:0"`
	DurationDays int    `json:"duration_days" gorm:"default:0"`
	Status       int    `json:"status" gorm:"default:1;index"`
	Sort         int    `json:"sort" gorm:"default:0;index"`
	CreatedTime  int64  `json:"created_time" gorm:"bigint"`
	UpdatedTime  int64  `json:"updated_time" gorm:"bigint"`
}

type PaymentOrder struct {
	Id              int    `json:"id"`
	OrderNo         string `json:"order_no" gorm:"uniqueIndex;size:64"`
	UserId          int    `json:"user_id" gorm:"index"`
	PackageId       int    `json:"package_id" gorm:"index"`
	PackageSnapshot string `json:"package_snapshot" gorm:"type:text"`
	Provider        string `json:"provider" gorm:"size:32;index"`
	ProviderTradeNo string `json:"provider_trade_no" gorm:"size:128;index"`
	AmountCents     int64  `json:"amount_cents" gorm:"bigint;default:0"`
	Quota           int64  `json:"quota" gorm:"bigint;default:0"`
	DurationDays    int    `json:"duration_days" gorm:"default:0"`
	Status          int    `json:"status" gorm:"default:1;index"`
	PaymentUrl      string `json:"payment_url" gorm:"type:text"`
	CreatedTime     int64  `json:"created_time" gorm:"bigint"`
	PaidTime        int64  `json:"paid_time" gorm:"bigint"`
	ClosedTime      int64  `json:"closed_time" gorm:"bigint"`
	RefundedTime    int64  `json:"refunded_time" gorm:"bigint"`
	RefundReason    string `json:"refund_reason" gorm:"type:text"`
}

type QuotaGrant struct {
	Id          int    `json:"id"`
	UserId      int    `json:"user_id" gorm:"index"`
	SourceType  string `json:"source_type" gorm:"size:32;index"`
	SourceId    int    `json:"source_id" gorm:"index"`
	Quota       int64  `json:"quota" gorm:"bigint;default:0"`
	RemainQuota int64  `json:"remain_quota" gorm:"bigint;default:0"`
	ExpiredTime int64  `json:"expired_time" gorm:"bigint;default:-1;index"`
	Status      int    `json:"status" gorm:"default:1;index"`
	CreatedTime int64  `json:"created_time" gorm:"bigint"`
	UpdatedTime int64  `json:"updated_time" gorm:"bigint"`
}

type PaymentEvent struct {
	Id              int    `json:"id"`
	Provider        string `json:"provider" gorm:"size:32;index"`
	OrderNo         string `json:"order_no" gorm:"size:64;index"`
	ProviderTradeNo string `json:"provider_trade_no" gorm:"size:128;index"`
	EventType       string `json:"event_type" gorm:"size:64"`
	RawPayload      string `json:"raw_payload" gorm:"type:text"`
	Status          int    `json:"status" gorm:"default:1;index"`
	CreatedTime     int64  `json:"created_time" gorm:"bigint"`
}

type PaymentOrderFilter struct {
	Keyword  string
	Provider string
	Status   int
	UserId   int
}

func (p *TopUpPackage) Normalize() {
	if p.Type == "" {
		p.Type = TopUpPackageTypeFixed
	}
	if p.Type == TopUpPackageTypeFixed {
		p.DurationDays = 0
	}
	now := helper.GetTimestamp()
	if p.CreatedTime == 0 {
		p.CreatedTime = now
	}
	p.UpdatedTime = now
}

func (p *TopUpPackage) Validate() error {
	if p.Name == "" || len(p.Name) > 50 {
		return errors.New("套餐名称长度必须在 1-50 之间")
	}
	if p.Type != TopUpPackageTypeFixed && p.Type != TopUpPackageTypeDuration {
		return errors.New("套餐类型必须是 fixed 或 duration")
	}
	if p.PriceCents <= 0 {
		return errors.New("套餐价格必须大于 0")
	}
	if p.Quota <= 0 {
		return errors.New("套餐额度必须大于 0")
	}
	if p.Type == TopUpPackageTypeDuration && p.DurationDays <= 0 {
		return errors.New("限期套餐必须设置有效天数")
	}
	if p.Status == 0 {
		p.Status = TopUpPackageStatusEnabled
	}
	return nil
}

func GetAllTopUpPackages(startIdx int, num int) ([]*TopUpPackage, error) {
	var packages []*TopUpPackage
	err := DB.Order("sort desc, id desc").Limit(num).Offset(startIdx).Find(&packages).Error
	return packages, err
}

func GetAvailableTopUpPackages() ([]*TopUpPackage, error) {
	var packages []*TopUpPackage
	err := DB.Where("status = ?", TopUpPackageStatusEnabled).Order("sort desc, id desc").Find(&packages).Error
	return packages, err
}

func GetTopUpPackageById(id int) (*TopUpPackage, error) {
	if id == 0 {
		return nil, errors.New("id 为空")
	}
	p := &TopUpPackage{}
	err := DB.First(p, "id = ?", id).Error
	return p, err
}

func (p *TopUpPackage) Insert() error {
	p.Normalize()
	if err := p.Validate(); err != nil {
		return err
	}
	return DB.Create(p).Error
}

func (p *TopUpPackage) Update() error {
	p.Normalize()
	if err := p.Validate(); err != nil {
		return err
	}
	return DB.Model(p).Select("name", "description", "type", "price_cents", "quota", "duration_days", "status", "sort", "updated_time").Updates(p).Error
}

func DeleteTopUpPackageById(id int) error {
	if id == 0 {
		return errors.New("id 为空")
	}
	return DB.Delete(&TopUpPackage{}, id).Error
}

func GeneratePaymentOrderNo() string {
	return fmt.Sprintf("O%d%s", helper.GetTimestamp(), random.GetRandomNumberString(8))
}

func CreatePaymentOrder(userId int, packageId int, provider string) (*PaymentOrder, error) {
	if userId == 0 {
		return nil, errors.New("无效的用户")
	}
	if provider != "wechat" && provider != "alipay" && provider != "mock" {
		return nil, errors.New("支付方式必须是 wechat、alipay 或 mock")
	}
	p, err := GetTopUpPackageById(packageId)
	if err != nil {
		return nil, err
	}
	if p.Status != TopUpPackageStatusEnabled {
		return nil, errors.New("套餐已下架")
	}
	snapshot, _ := json.Marshal(p)
	order := &PaymentOrder{
		OrderNo:         GeneratePaymentOrderNo(),
		UserId:          userId,
		PackageId:       p.Id,
		PackageSnapshot: string(snapshot),
		Provider:        provider,
		AmountCents:     p.PriceCents,
		Quota:           p.Quota,
		DurationDays:    p.DurationDays,
		Status:          PaymentOrderStatusPending,
		CreatedTime:     helper.GetTimestamp(),
	}
	err = DB.Create(order).Error
	return order, err
}

func GetPaymentOrderById(id int, userId int, admin bool) (*PaymentOrder, error) {
	order := &PaymentOrder{}
	query := DB.Where("id = ?", id)
	if !admin {
		query = query.Where("user_id = ?", userId)
	}
	err := query.First(order).Error
	return order, err
}

func GetPaymentOrderByOrderNo(orderNo string) (*PaymentOrder, error) {
	order := &PaymentOrder{}
	err := DB.First(order, "order_no = ?", orderNo).Error
	return order, err
}

func GetUserPaymentOrders(userId int, startIdx int, num int) ([]*PaymentOrder, error) {
	var orders []*PaymentOrder
	err := DB.Where("user_id = ?", userId).Order("id desc").Limit(num).Offset(startIdx).Find(&orders).Error
	return orders, err
}

func GetAllPaymentOrders(filter PaymentOrderFilter, startIdx int, num int) ([]*PaymentOrder, error) {
	var orders []*PaymentOrder
	query := DB.Model(&PaymentOrder{})
	if filter.Keyword != "" {
		keyword := "%" + filter.Keyword + "%"
		query = query.Where("order_no LIKE ? or provider_trade_no LIKE ?", keyword, keyword)
	}
	if filter.Provider != "" {
		query = query.Where("provider = ?", filter.Provider)
	}
	if filter.Status > 0 {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.UserId > 0 {
		query = query.Where("user_id = ?", filter.UserId)
	}
	err := query.Order("id desc").Limit(num).Offset(startIdx).Find(&orders).Error
	return orders, err
}

func UpdatePaymentOrderUrl(id int, paymentUrl string) error {
	return DB.Model(&PaymentOrder{}).Where("id = ? and status = ?", id, PaymentOrderStatusPending).Update("payment_url", paymentUrl).Error
}

func ClosePaymentOrder(id int, userId int, admin bool) error {
	query := DB.Model(&PaymentOrder{}).Where("id = ? and status = ?", id, PaymentOrderStatusPending)
	if !admin {
		query = query.Where("user_id = ?", userId)
	}
	return query.Updates(map[string]interface{}{
		"status":      PaymentOrderStatusClosed,
		"closed_time": helper.GetTimestamp(),
	}).Error
}

func RefundPaymentOrder(ctx context.Context, id int, reason string) (*PaymentOrder, error) {
	if id == 0 {
		return nil, errors.New("id 为空")
	}
	var order *PaymentOrder
	wasAlreadyRefunded := false
	err := DB.Transaction(func(tx *gorm.DB) error {
		current := &PaymentOrder{}
		if err := tx.Set("gorm:query_option", "FOR UPDATE").First(current, "id = ?", id).Error; err != nil {
			return err
		}
		if current.Status == PaymentOrderStatusRefunded {
			order = current
			wasAlreadyRefunded = true
			return nil
		}
		if current.Status != PaymentOrderStatusPaid {
			return errors.New("只有已支付订单可以退款作废")
		}
		now := helper.GetTimestamp()
		if err := tx.Model(&User{}).Where("id = ?", current.UserId).Update("quota", gorm.Expr("quota - ?", current.Quota)).Error; err != nil {
			return err
		}
		if err := tx.Model(&QuotaGrant{}).Where("source_id = ? and source_type in ?", current.Id, []string{"fixed_package", "duration_package"}).Updates(map[string]interface{}{
			"remain_quota": 0,
			"status":       QuotaGrantStatusExpired,
			"updated_time": now,
		}).Error; err != nil {
			return err
		}
		if err := tx.Model(current).Updates(map[string]interface{}{
			"status":        PaymentOrderStatusRefunded,
			"refunded_time": now,
			"refund_reason": reason,
		}).Error; err != nil {
			return err
		}
		current.Status = PaymentOrderStatusRefunded
		current.RefundedTime = now
		current.RefundReason = reason
		order = current
		return nil
	})
	if err != nil {
		return nil, err
	}
	if !wasAlreadyRefunded {
		RecordTopupLog(ctx, order.UserId, fmt.Sprintf("管理员退款作废订单 %s，扣回 %s", order.OrderNo, common.LogQuota(order.Quota)), -int(order.Quota))
	}
	return order, nil
}

func RecordPaymentEvent(provider string, orderNo string, providerTradeNo string, eventType string, rawPayload string, status int) error {
	event := &PaymentEvent{
		Provider:        provider,
		OrderNo:         orderNo,
		ProviderTradeNo: providerTradeNo,
		EventType:       eventType,
		RawPayload:      rawPayload,
		Status:          status,
		CreatedTime:     helper.GetTimestamp(),
	}
	return DB.Create(event).Error
}

func createQuotaGrantTx(tx *gorm.DB, userId int, sourceType string, sourceId int, quota int64, durationDays int) error {
	now := helper.GetTimestamp()
	expiredTime := int64(-1)
	if durationDays > 0 {
		expiredTime = now + int64(durationDays)*24*60*60
	}
	grant := &QuotaGrant{
		UserId:      userId,
		SourceType:  sourceType,
		SourceId:    sourceId,
		Quota:       quota,
		RemainQuota: quota,
		ExpiredTime: expiredTime,
		Status:      QuotaGrantStatusActive,
		CreatedTime: now,
		UpdatedTime: now,
	}
	return tx.Create(grant).Error
}

func AddPermanentQuotaGrant(userId int, sourceType string, sourceId int, quota int64) error {
	if userId == 0 || quota <= 0 {
		return nil
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		return createQuotaGrantTx(tx, userId, sourceType, sourceId, quota, 0)
	})
}

func consumeQuotaGrantsTx(tx *gorm.DB, userId int, quota int64) error {
	if userId == 0 || quota <= 0 {
		return nil
	}
	var grants []*QuotaGrant
	now := helper.GetTimestamp()
	err := tx.Where("user_id = ? and status = ? and remain_quota > 0 and (expired_time = -1 or expired_time > ?)", userId, QuotaGrantStatusActive, now).
		Order("case when expired_time = -1 then 1 else 0 end asc, expired_time asc, id asc").
		Find(&grants).Error
	if err != nil {
		return err
	}
	remaining := quota
	for _, grant := range grants {
		if remaining <= 0 {
			break
		}
		used := grant.RemainQuota
		if used > remaining {
			used = remaining
		}
		updates := map[string]interface{}{
			"remain_quota": gorm.Expr("remain_quota - ?", used),
			"updated_time": helper.GetTimestamp(),
		}
		if grant.RemainQuota-used <= 0 {
			updates["status"] = QuotaGrantStatusExpired
		}
		if err := tx.Model(&QuotaGrant{}).Where("id = ?", grant.Id).Updates(updates).Error; err != nil {
			return err
		}
		remaining -= used
	}
	return nil
}

func ConsumeQuotaGrants(userId int, quota int64) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		return consumeQuotaGrantsTx(tx, userId, quota)
	})
}

func ExpireUserQuotaGrants(userId int) error {
	if userId == 0 {
		return nil
	}
	now := helper.GetTimestamp()
	return DB.Transaction(func(tx *gorm.DB) error {
		var grants []*QuotaGrant
		err := tx.Where("user_id = ? and status = ? and expired_time > 0 and expired_time <= ? and remain_quota > 0", userId, QuotaGrantStatusActive, now).Find(&grants).Error
		if err != nil {
			return err
		}
		var expiredQuota int64
		for _, grant := range grants {
			expiredQuota += grant.RemainQuota
		}
		if expiredQuota > 0 {
			if err := tx.Model(&User{}).Where("id = ?", userId).Update("quota", gorm.Expr("case when quota >= ? then quota - ? else 0 end", expiredQuota, expiredQuota)).Error; err != nil {
				return err
			}
		}
		return tx.Model(&QuotaGrant{}).Where("user_id = ? and status = ? and expired_time > 0 and expired_time <= ?", userId, QuotaGrantStatusActive, now).Updates(map[string]interface{}{
			"remain_quota": 0,
			"status":       QuotaGrantStatusExpired,
			"updated_time": helper.GetTimestamp(),
		}).Error
	})
}

func CompletePaymentOrder(ctx context.Context, orderNo string, provider string, providerTradeNo string) (*PaymentOrder, error) {
	order, err := GetPaymentOrderByOrderNo(orderNo)
	if err != nil {
		return nil, err
	}
	if order.Provider != provider {
		return nil, errors.New("支付渠道不匹配")
	}
	wasAlreadyPaid := false
	err = DB.Transaction(func(tx *gorm.DB) error {
		current := &PaymentOrder{}
		if err := tx.Set("gorm:query_option", "FOR UPDATE").First(current, "order_no = ?", orderNo).Error; err != nil {
			return err
		}
		if current.Status == PaymentOrderStatusPaid {
			order = current
			wasAlreadyPaid = true
			return nil
		}
		if current.Status != PaymentOrderStatusPending {
			return errors.New("订单状态不可支付")
		}
		now := helper.GetTimestamp()
		sourceType := "fixed_package"
		if current.DurationDays > 0 {
			sourceType = "duration_package"
		}
		if err := createQuotaGrantTx(tx, current.UserId, sourceType, current.Id, current.Quota, current.DurationDays); err != nil {
			return err
		}
		if err := tx.Model(&User{}).Where("id = ?", current.UserId).Update("quota", gorm.Expr("quota + ?", current.Quota)).Error; err != nil {
			return err
		}
		if err := tx.Model(current).Updates(map[string]interface{}{
			"status":            PaymentOrderStatusPaid,
			"provider_trade_no": providerTradeNo,
			"paid_time":         now,
		}).Error; err != nil {
			return err
		}
		current.Status = PaymentOrderStatusPaid
		current.ProviderTradeNo = providerTradeNo
		current.PaidTime = now
		order = current
		return nil
	})
	if err != nil {
		return nil, err
	}
	if !wasAlreadyPaid {
		RecordTopupLog(ctx, order.UserId, fmt.Sprintf("通过订单 %s 充值 %s", order.OrderNo, common.LogQuota(order.Quota)), int(order.Quota))
	}
	return order, nil
}

func GetUserQuotaGrants(userId int) ([]*QuotaGrant, error) {
	_ = ExpireUserQuotaGrants(userId)
	var grants []*QuotaGrant
	err := DB.Where("user_id = ?", userId).Order("status asc, expired_time asc, id desc").Find(&grants).Error
	return grants, err
}
