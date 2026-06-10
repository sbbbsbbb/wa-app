package app

import (
	"context"
	"strings"
	"time"

	waappv1 "github.com/byte-v-forge/wa-app/gen/go/byte/v/forge/waapp/v1"
)

type waTextMessageSender interface {
	SendTextMessage(context.Context, EngineTextMessageInput) EngineTextMessageResult
}

func (s *Server) SendTextMessage(ctx context.Context, req *waappv1.SendTextMessageRequest) (*waappv1.SendTextMessageResponse, error) {
	if err := validateContext(req.GetContext()); err != nil {
		return &waappv1.SendTextMessageResponse{Error: ToProtoError(err)}, nil
	}
	accountID, err := requireWAAccountID(req.GetWaAccountId())
	if err != nil {
		return &waappv1.SendTextMessageResponse{Error: ToProtoError(err)}, nil
	}
	if _, err := s.getWAAccount(ctx, accountID); err != nil {
		return &waappv1.SendTextMessageResponse{Error: ToProtoError(err)}, nil
	}
	text := strings.TrimSpace(req.GetText().GetValue())
	if text == "" {
		return &waappv1.SendTextMessageResponse{Error: ToProtoError(NewError(waappv1.WaErrorCode_WA_ERROR_CODE_VALIDATION_FAILED, "text is required", false))}, nil
	}
	contactJID := s.textMessageContactJID(ctx, accountID, req.GetContactRef())
	if contactJID == "" {
		return &waappv1.SendTextMessageResponse{Error: ToProtoError(NewError(waappv1.WaErrorCode_WA_ERROR_CODE_VALIDATION_FAILED, "contact_ref is required", false))}, nil
	}
	loginState, err := s.activeContactResolveLoginState(ctx, accountID)
	if err != nil {
		return &waappv1.SendTextMessageResponse{Error: ToProtoError(err)}, nil
	}
	runner, release, err := s.textMessageRunner(ctx, req.GetContext(), loginState)
	if err != nil {
		return &waappv1.SendTextMessageResponse{Error: ToProtoError(err)}, nil
	}
	defer release()
	sender, ok := runner.(waTextMessageSender)
	if !ok {
		return &waappv1.SendTextMessageResponse{Error: ToProtoError(NewError(waappv1.WaErrorCode_WA_ERROR_CODE_UNSUPPORTED_OPERATION, "WA text message sender is not configured", false))}, nil
	}
	result := sender.SendTextMessage(ctx, EngineTextMessageInput{
		WAAccountID:          accountID,
		ClientProfileID:      loginState.GetClientProfileId(),
		RegisteredIdentityID: loginState.GetRegisteredIdentityId(),
		ContactJID:           contactJID,
		Text:                 text,
		ClientMessageID:      req.GetClientMessageId(),
		RemoteTimeout:        defaultTextMessageSendTimeout,
	})
	if result.Err != nil {
		return &waappv1.SendTextMessageResponse{ProviderMessageId: result.ProviderMessageID, SentAt: timestamp(result.SentAt), Error: ToProtoError(result.Err)}, nil
	}
	return &waappv1.SendTextMessageResponse{ProviderMessageId: result.ProviderMessageID, SentAt: timestampOrNow(result.SentAt, s.clock.Now())}, nil
}

func (s *Server) textMessageContactJID(ctx context.Context, accountID string, contactRef string) string {
	contactRef = strings.TrimSpace(contactRef)
	if contactRef == "" {
		return ""
	}
	contact, err := s.store.GetWAContactByRef(ctx, accountID, contactRef)
	if err == nil && contact.GetWaAccountId() == accountID {
		if jid := normalizeWAJID(contact.GetJid()); jid != "" {
			return jid
		}
		if number := strings.TrimSpace(contact.GetNumber()); number != "" {
			return normalizeWAJID(number)
		}
	}
	return normalizeWAJID(contactRef)
}

func (s *Server) textMessageRunner(ctx context.Context, requestContext *waappv1.RequestContext, loginState *waappv1.LoginState) (ProtocolEngine, func(), error) {
	if s.longConnections != nil {
		if runner := s.longConnections.Runner(loginState); runner != nil {
			return runner, func() {}, nil
		}
	}
	runner := s.runner
	native, ok := runner.(*NativeEngine)
	if !ok {
		return runner, func() {}, nil
	}
	proxied, release, _ := s.optionalGatewayProxyEngine(ctx, native, gatewayProxyEngineRequest{
		Username:      s.longProxyUsername,
		Purpose:       "WA_MESSAGE_SEND",
		CorrelationID: firstNonEmpty(requestContext.GetCorrelationId(), requestContext.GetRequestId()),
		TTL:           defaultTextMessageSendTimeout + 10*time.Second,
		Mode:          DynamicProxySessionModeSticky,
	})
	return proxied, release, nil
}
