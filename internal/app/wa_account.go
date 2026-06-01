package app

import (
	"strings"
	"time"

	"github.com/byte-v-forge/common-lib/accountmodel"
	accountv1 "github.com/byte-v-forge/common-lib/gen/go/byte/v/forge/contracts/account/v1"
	waappv1 "github.com/byte-v-forge/wa-app/gen/go/byte/v/forge/waapp/v1"
)

var waAccountDescriptor = accountmodel.Descriptor{SourceService: waPlatformEventSource, AccountType: "wa", ProviderKey: "whatsapp"}

func newWAAccount(id string, workspaceID string, phone *waappv1.PhoneTarget, status waappv1.WAAccountStatus, audit *waappv1.AuditStamp) *waappv1.WAAccount {
	phone = normalizePhone(phone)
	return &waappv1.WAAccount{
		Account: waAccountDescriptor.Account(
			id,
			accountmodel.WithPhoneIdentity(phone.GetE164Number(), phone.GetE164Number()),
			accountmodel.WithStatus(waAccountStatusModel(status)),
			accountmodel.WithCreatedTimestamp(audit.GetCreatedAt()),
			accountmodel.WithUpdatedTimestamp(audit.GetUpdatedAt()),
		),
		WorkspaceId: workspaceID,
		Phone:       phone,
	}
}

func waAccountStatusModel(status waappv1.WAAccountStatus) *accountv1.AccountStatus {
	switch status {
	case waappv1.WAAccountStatus_WA_ACCOUNT_STATUS_PENDING_REGISTRATION:
		return accountmodel.Status("pending_registration", "待注册", nil)
	case waappv1.WAAccountStatus_WA_ACCOUNT_STATUS_ACTIVE:
		return accountmodel.Status("active", "已注册", nil)
	case waappv1.WAAccountStatus_WA_ACCOUNT_STATUS_PAUSED:
		return accountmodel.Status("paused", "暂停", nil)
	case waappv1.WAAccountStatus_WA_ACCOUNT_STATUS_ARCHIVED:
		return accountmodel.Status("archived", "归档", nil)
	default:
		return accountmodel.StatusFromStringer(status, "WA_ACCOUNT_STATUS_")
	}
}

func withWAAccountStatus(account *waappv1.WAAccount, status waappv1.WAAccountStatus, updatedAt time.Time) *waappv1.WAAccount {
	createdAt := waAccountCreatedAt(account)
	if createdAt.IsZero() {
		createdAt = updatedAt
	}
	return newWAAccount(waAccountID(account), account.GetWorkspaceId(), account.GetPhone(), status, audit(createdAt, updatedAt))
}

func waAccountID(account *waappv1.WAAccount) string {
	return accountmodel.AccountID(account.GetAccount())
}

func waAccountStatus(account *waappv1.WAAccount) waappv1.WAAccountStatus {
	value := strings.ToUpper(accountmodel.StatusValue(account.GetAccount()))
	if value == "" {
		return waappv1.WAAccountStatus_WA_ACCOUNT_STATUS_UNSPECIFIED
	}
	if !strings.HasPrefix(value, "WA_ACCOUNT_STATUS_") {
		value = "WA_ACCOUNT_STATUS_" + value
	}
	return waappv1.WAAccountStatus(waappv1.WAAccountStatus_value[value])
}

func waAccountStatusStorageValue(account *waappv1.WAAccount) string {
	return waAccountStatus(account).String()
}

func waAccountCreatedAt(account *waappv1.WAAccount) time.Time {
	return accountmodel.TimestampTime(account.GetAccount().GetCreatedAt())
}

func waAccountUpdatedAt(account *waappv1.WAAccount) time.Time {
	return accountmodel.TimestampTime(account.GetAccount().GetUpdatedAt())
}

func requireWAAccountID(value string) (string, error) {
	accountID, err := waAccountDescriptor.NormalizeID(value, "wa_account_id")
	if err != nil {
		return "", NewError(waappv1.WaErrorCode_WA_ERROR_CODE_VALIDATION_FAILED, err.Error(), false)
	}
	return accountID, nil
}
