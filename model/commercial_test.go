package model

import (
	"context"
	"strings"
	"testing"

	"github.com/songquanpeng/one-api/common"
	"github.com/songquanpeng/one-api/common/config"
	"github.com/songquanpeng/one-api/common/helper"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupCommercialTestDB(t *testing.T) {
	t.Helper()

	oldDB := DB
	oldLogDB := LOG_DB
	oldUsingSQLite := common.UsingSQLite
	oldUsingMySQL := common.UsingMySQL
	oldUsingPostgreSQL := common.UsingPostgreSQL
	oldBatchUpdateEnabled := config.BatchUpdateEnabled

	dsnName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+dsnName+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)

	DB = db
	LOG_DB = db
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	config.BatchUpdateEnabled = false

	require.NoError(t, DB.AutoMigrate(&User{}, &TopUpPackage{}, &PaymentOrder{}, &QuotaGrant{}, &PaymentEvent{}, &Log{}))

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
		DB = oldDB
		LOG_DB = oldLogDB
		common.UsingSQLite = oldUsingSQLite
		common.UsingMySQL = oldUsingMySQL
		common.UsingPostgreSQL = oldUsingPostgreSQL
		config.BatchUpdateEnabled = oldBatchUpdateEnabled
	})
}

func createCommercialTestUser(t *testing.T, quota int64) *User {
	t.Helper()

	user := &User{
		Username:    "user",
		Password:    "password",
		DisplayName: "Test User",
		Role:        RoleCommonUser,
		Status:      UserStatusEnabled,
		AccessToken: "test-access-token",
		AffCode:     "test-aff",
		Quota:       quota,
		Group:       "default",
	}
	require.NoError(t, DB.Create(user).Error)
	return user
}

func createCommercialTestPackage(t *testing.T, packageType string, quota int64, durationDays int) *TopUpPackage {
	t.Helper()

	pkg := &TopUpPackage{
		Name:         "Test Package",
		Type:         packageType,
		PriceCents:   990,
		Quota:        quota,
		DurationDays: durationDays,
		Status:       TopUpPackageStatusEnabled,
	}
	require.NoError(t, pkg.Insert())
	return pkg
}

func TestCompleteMockPaymentOrderGrantsQuotaOnce(t *testing.T) {
	setupCommercialTestDB(t)
	user := createCommercialTestUser(t, 1000)
	pkg := createCommercialTestPackage(t, TopUpPackageTypeFixed, 500, 0)

	order, err := CreatePaymentOrder(user.Id, pkg.Id, "mock")
	require.NoError(t, err)

	completed, err := CompletePaymentOrder(context.Background(), order.OrderNo, "mock", "trade-1")
	require.NoError(t, err)
	require.Equal(t, PaymentOrderStatusPaid, completed.Status)
	require.Equal(t, "trade-1", completed.ProviderTradeNo)

	quota, err := GetUserQuota(user.Id)
	require.NoError(t, err)
	require.Equal(t, int64(1500), quota)

	var grant QuotaGrant
	require.NoError(t, DB.First(&grant, "source_type = ? and source_id = ?", "fixed_package", order.Id).Error)
	require.Equal(t, int64(500), grant.Quota)
	require.Equal(t, int64(500), grant.RemainQuota)
	require.Equal(t, int64(-1), grant.ExpiredTime)
	require.Equal(t, QuotaGrantStatusActive, grant.Status)

	_, err = CompletePaymentOrder(context.Background(), order.OrderNo, "mock", "trade-1")
	require.NoError(t, err)

	quota, err = GetUserQuota(user.Id)
	require.NoError(t, err)
	require.Equal(t, int64(1500), quota)

	var grantCount int64
	require.NoError(t, DB.Model(&QuotaGrant{}).Where("source_id = ?", order.Id).Count(&grantCount).Error)
	require.Equal(t, int64(1), grantCount)

	var logCount int64
	require.NoError(t, DB.Model(&Log{}).Where("user_id = ? and type = ?", user.Id, LogTypeTopup).Count(&logCount).Error)
	require.Equal(t, int64(1), logCount)
}

func TestDurationQuotaGrantExpiresAndAdjustsQuota(t *testing.T) {
	setupCommercialTestDB(t)
	user := createCommercialTestUser(t, 1000)
	pkg := createCommercialTestPackage(t, TopUpPackageTypeDuration, 200, 1)

	order, err := CreatePaymentOrder(user.Id, pkg.Id, "mock")
	require.NoError(t, err)
	_, err = CompletePaymentOrder(context.Background(), order.OrderNo, "mock", "trade-1")
	require.NoError(t, err)

	quota, err := GetUserQuota(user.Id)
	require.NoError(t, err)
	require.Equal(t, int64(1200), quota)

	var grant QuotaGrant
	require.NoError(t, DB.First(&grant, "source_type = ? and source_id = ?", "duration_package", order.Id).Error)
	require.Greater(t, grant.ExpiredTime, helper.GetTimestamp())

	require.NoError(t, DB.Model(&QuotaGrant{}).Where("id = ?", grant.Id).Updates(map[string]interface{}{
		"expired_time": helper.GetTimestamp() - 1,
		"remain_quota": 200,
		"status":       QuotaGrantStatusActive,
	}).Error)
	require.NoError(t, ExpireUserQuotaGrants(user.Id))

	quota, err = GetUserQuota(user.Id)
	require.NoError(t, err)
	require.Equal(t, int64(1000), quota)

	require.NoError(t, DB.First(&grant, grant.Id).Error)
	require.Equal(t, int64(0), grant.RemainQuota)
	require.Equal(t, QuotaGrantStatusExpired, grant.Status)
}

func TestDecreaseUserQuotaConsumesExpiringGrantFirst(t *testing.T) {
	setupCommercialTestDB(t)
	user := createCommercialTestUser(t, 600)
	now := helper.GetTimestamp()

	durationGrant := &QuotaGrant{
		UserId:      user.Id,
		SourceType:  "duration_package",
		SourceId:    1,
		Quota:       300,
		RemainQuota: 300,
		ExpiredTime: now + 3600,
		Status:      QuotaGrantStatusActive,
		CreatedTime: now,
		UpdatedTime: now,
	}
	permanentGrant := &QuotaGrant{
		UserId:      user.Id,
		SourceType:  "fixed_package",
		SourceId:    2,
		Quota:       300,
		RemainQuota: 300,
		ExpiredTime: -1,
		Status:      QuotaGrantStatusActive,
		CreatedTime: now,
		UpdatedTime: now,
	}
	require.NoError(t, DB.Create(durationGrant).Error)
	require.NoError(t, DB.Create(permanentGrant).Error)

	require.NoError(t, DecreaseUserQuota(user.Id, 350))

	quota, err := GetUserQuota(user.Id)
	require.NoError(t, err)
	require.Equal(t, int64(250), quota)

	require.NoError(t, DB.First(durationGrant, durationGrant.Id).Error)
	require.Equal(t, int64(0), durationGrant.RemainQuota)
	require.Equal(t, QuotaGrantStatusExpired, durationGrant.Status)

	require.NoError(t, DB.First(permanentGrant, permanentGrant.Id).Error)
	require.Equal(t, int64(250), permanentGrant.RemainQuota)
	require.Equal(t, QuotaGrantStatusActive, permanentGrant.Status)
}

func TestRefundPaymentOrderRollsBackQuotaOnce(t *testing.T) {
	setupCommercialTestDB(t)
	user := createCommercialTestUser(t, 1000)
	pkg := createCommercialTestPackage(t, TopUpPackageTypeFixed, 500, 0)

	order, err := CreatePaymentOrder(user.Id, pkg.Id, "mock")
	require.NoError(t, err)
	_, err = CompletePaymentOrder(context.Background(), order.OrderNo, "mock", "trade-1")
	require.NoError(t, err)

	require.NoError(t, DecreaseUserQuota(user.Id, 200))

	refunded, err := RefundPaymentOrder(context.Background(), order.Id, "duplicate payment")
	require.NoError(t, err)
	require.Equal(t, PaymentOrderStatusRefunded, refunded.Status)
	require.Equal(t, "duplicate payment", refunded.RefundReason)

	quota, err := GetUserQuota(user.Id)
	require.NoError(t, err)
	require.Equal(t, int64(800), quota)

	var grant QuotaGrant
	require.NoError(t, DB.First(&grant, "source_id = ?", order.Id).Error)
	require.Equal(t, int64(0), grant.RemainQuota)
	require.Equal(t, QuotaGrantStatusExpired, grant.Status)

	_, err = RefundPaymentOrder(context.Background(), order.Id, "second click")
	require.NoError(t, err)

	quota, err = GetUserQuota(user.Id)
	require.NoError(t, err)
	require.Equal(t, int64(800), quota)

	var logCount int64
	require.NoError(t, DB.Model(&Log{}).Where("user_id = ? and type = ?", user.Id, LogTypeTopup).Count(&logCount).Error)
	require.Equal(t, int64(2), logCount)
}
