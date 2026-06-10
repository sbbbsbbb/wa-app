package app

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"sort"
	"strings"
	"time"

	waappv1 "github.com/byte-v-forge/wa-app/gen/go/byte/v/forge/waapp/v1"
)

const defaultTextMessageSendTimeout = 20 * time.Second

func (e *NativeEngine) SendTextMessage(ctx context.Context, input EngineTextMessageInput) EngineTextMessageResult {
	if e == nil {
		return EngineTextMessageResult{Err: NewError(waappv1.WaErrorCode_WA_ERROR_CODE_INTERNAL, "native engine is required", false)}
	}
	state, err := e.loadState(ctx, input.ClientProfileID)
	if err != nil {
		return EngineTextMessageResult{Err: err}
	}
	state.ensureMaps()
	state.ChatStatic = ensureChatStatic(state.ChatStatic)
	proxyURL, err := e.proxyURL()
	if err != nil {
		return EngineTextMessageResult{Err: err}
	}
	timeout := textMessageSendTimeout(input.RemoteTimeout)
	operationCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	client := newChatdClient(chatdConfigForState(proxyURL, state, timeout))
	session, err := client.openSession(operationCtx, state, input.RegisteredIdentityID, defaultLoginPayload, defaultWAAppVersion)
	if err != nil {
		return EngineTextMessageResult{Err: chatdReceiveError(err)}
	}
	defer session.Close()
	result := e.sendTextMessageOnSession(operationCtx, session, &state, input, timeout)
	return result
}

func (e *longConnectionNativeEngine) SendTextMessage(ctx context.Context, input EngineTextMessageInput) EngineTextMessageResult {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return EngineTextMessageResult{Err: NewError(waappv1.WaErrorCode_WA_ERROR_CODE_CONFLICT, "WA long connection runner is closed", true)}
	}
	state, err := e.loadState(ctx, input.ClientProfileID)
	if err != nil {
		return EngineTextMessageResult{Err: err}
	}
	state.ensureMaps()
	state.ChatStatic = ensureChatStatic(state.ChatStatic)
	messageInput := EngineMessageInput{WAAccountID: input.WAAccountID, ClientProfileID: input.ClientProfileID, RegisteredIdentityID: input.RegisteredIdentityID}
	session, err := e.ensureSessionForIQLocked(ctx, messageInput, state)
	if err != nil {
		e.closeLocked()
		return EngineTextMessageResult{Err: chatdReceiveError(err)}
	}
	timeout := contextBoundTimeout(ctx, textMessageSendTimeout(input.RemoteTimeout))
	result := e.NativeEngine.sendTextMessageOnSession(ctx, session, &state, input, timeout)
	if result.Err != nil && isChatdSendError(result.Err) {
		e.closeLocked()
	}
	return result
}

func (e *NativeEngine) sendTextMessageOnSession(ctx context.Context, session *chatdSession, state *nativeState, input EngineTextMessageInput, timeout time.Duration) EngineTextMessageResult {
	providerID := newTextProviderMessageID(input.ClientMessageID)
	sentAt := e.clock.Now()
	node, err := buildNativeTextMessageNode(state, input, providerID)
	if err != nil {
		return EngineTextMessageResult{ProviderMessageID: providerID, SentAt: sentAt, Err: err}
	}
	applyChatdSessionUpdateState(state, session.update())
	if err := e.saveState(ctx, input.ClientProfileID, *state); err != nil {
		return EngineTextMessageResult{ProviderMessageID: providerID, SentAt: sentAt, Err: err}
	}
	if err := sendChatdNodeWithTimeout(ctx, session, node, timeout); err != nil {
		return EngineTextMessageResult{ProviderMessageID: providerID, SentAt: sentAt, Err: chatdSendError(err)}
	}
	return EngineTextMessageResult{ProviderMessageID: providerID, SentAt: sentAt}
}

func buildNativeTextMessageNode(state *nativeState, input EngineTextMessageInput, providerID string) (chatdNode, error) {
	contactJID := normalizeWAJID(input.ContactJID)
	text := strings.TrimSpace(input.Text)
	if contactJID == "" {
		return chatdNode{}, NewError(waappv1.WaErrorCode_WA_ERROR_CODE_VALIDATION_FAILED, "contact_ref is required", false)
	}
	if text == "" {
		return chatdNode{}, NewError(waappv1.WaErrorCode_WA_ERROR_CODE_VALIDATION_FAILED, "text is required", false)
	}
	ciphertext, err := encryptNativeTextSignalMessage(state, contactJID, text)
	if err != nil {
		return chatdNode{}, err
	}
	return chatdNode{
		Tag:   "message",
		Attrs: map[string]string{"id": providerID, "to": contactJID, "type": "text"},
		Content: []chatdNode{{
			Tag:     "enc",
			Attrs:   map[string]string{"type": "msg", "v": "2"},
			Content: ciphertext,
		}},
	}, nil
}

func encryptNativeTextSignalMessage(state *nativeState, contactJID string, text string) ([]byte, error) {
	state.ensureMaps()
	key, session, ok := exactSignalSession(state.Signal.Sessions, contactJID)
	if !ok {
		return nil, NewError(waappv1.WaErrorCode_WA_ERROR_CODE_UNSUPPORTED_OPERATION, "WA text send requires an existing Signal session for this contact", false)
	}
	if err := ensureOutboundSignalChain(&session); err != nil {
		return nil, err
	}
	raw, err := encryptSignalPlaintext(state, &session, nativeTextMessagePlaintext(text))
	if err != nil {
		return nil, err
	}
	state.Signal.Sessions[key] = session
	return raw, nil
}

func exactSignalSession(sessions map[string]nativeSignalSession, contactJID string) (string, nativeSignalSession, bool) {
	for _, candidate := range uniqueStrings(contactJID, normalizeWAJID(contactJID)) {
		key := signalSenderKey(candidate)
		if session, ok := sessions[key]; ok {
			return key, session, true
		}
	}
	return "", nativeSignalSession{}, false
}

func ensureOutboundSignalChain(session *nativeSignalSession) error {
	if session.SenderChain != nil && session.SenderChain.ChainKey != "" && session.SenderRatchetPrivate != "" {
		return nil
	}
	if session.RootKey == "" {
		return NewError(waappv1.WaErrorCode_WA_ERROR_CODE_UNSUPPORTED_OPERATION, "WA text send requires a learned Signal root key", false)
	}
	remoteRatchet, err := latestReceiverRatchetKey(*session)
	if err != nil {
		return err
	}
	rootKey, err := decodeB64Any(session.RootKey)
	if err != nil {
		return err
	}
	localRatchet, err := newNativeCurveKeyPair()
	if err != nil {
		return err
	}
	localPrivate, err := localRatchet.privateBytes()
	if err != nil {
		return err
	}
	localPublic, err := localRatchet.publicBytes()
	if err != nil {
		return err
	}
	nextRoot, chainKey, err := rootRatchet(rootKey, remoteRatchet, localPrivate)
	if err != nil {
		return err
	}
	session.RootKey = b64u(nextRoot)
	session.SenderRatchetPrivate = b64u(localPrivate)
	session.SenderRatchetPublic = b64u(localPublic)
	session.SenderChain = &nativeSenderChain{RatchetKey: b64u(localPublic), ChainKey: b64u(chainKey)}
	return nil
}

func latestReceiverRatchetKey(session nativeSignalSession) ([]byte, error) {
	if len(session.ReceiverChains) == 0 {
		return nil, NewError(waappv1.WaErrorCode_WA_ERROR_CODE_UNSUPPORTED_OPERATION, "WA text send requires a learned receiver chain", false)
	}
	keys := make([]string, 0, len(session.ReceiverChains))
	for key := range session.ReceiverChains {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var selected nativeReceiverChain
	selectedOK := false
	for _, key := range keys {
		chain := session.ReceiverChains[key]
		if !selectedOK || chain.Index >= selected.Index {
			selected = chain
			selectedOK = true
		}
	}
	if selected.RatchetKey != "" {
		return decodeB64Any(selected.RatchetKey)
	}
	for _, key := range keys {
		if raw, err := hex.DecodeString(key); err == nil && len(raw) > 0 {
			return raw, nil
		}
	}
	return nil, NewError(waappv1.WaErrorCode_WA_ERROR_CODE_UNSUPPORTED_OPERATION, "WA text send receiver ratchet is unavailable", false)
}

func encryptSignalPlaintext(state *nativeState, session *nativeSignalSession, plaintext []byte) ([]byte, error) {
	if session.SenderChain == nil {
		return nil, fmt.Errorf("missing sender chain")
	}
	chainKey, err := decodeB64Any(session.SenderChain.ChainKey)
	if err != nil {
		return nil, err
	}
	ratchetPublic, err := decodeB64Any(firstNonEmpty(session.SenderChain.RatchetKey, session.SenderRatchetPublic))
	if err != nil {
		return nil, err
	}
	ratchetPublic, err = stripSignalCurvePrefix(ratchetPublic)
	if err != nil {
		return nil, err
	}
	version := signalMessageVersion(session.Version)
	counter := session.SenderChain.Index
	keys := deriveMessageKeys(chainKey, counter)
	ciphertext, err := aesCBCPKCS7Encrypt(plaintext, keys.cipherKey, keys.iv)
	if err != nil {
		return nil, err
	}
	body := []byte{byte(version<<4 | version)}
	body = protoBytesInto(body, 1, ratchetPublic)
	body = protoVarintInto(body, 2, uint64(counter))
	body = protoBytesInto(body, 4, ciphertext)
	identityPublic, err := state.Signal.Identity.publicBytes()
	if err != nil {
		return nil, err
	}
	remoteIdentity, err := decodeB64Any(session.RemoteIdentityPublic)
	if err != nil {
		return nil, err
	}
	mac, err := signalMessageMAC(keys.macKey, identityPublic, remoteIdentity, body, version)
	if err != nil {
		return nil, err
	}
	session.SenderChain.ChainKey = b64u(nextChainKey(chainKey))
	session.SenderChain.Index = counter + 1
	return append(body, mac...), nil
}

func nativeTextMessagePlaintext(text string) []byte {
	return protoBytes(1, []byte(text))
}

func signalMessageVersion(version int) int {
	if version == 3 || version == 4 {
		return version
	}
	return 3
}

func sendChatdNodeWithTimeout(ctx context.Context, session *chatdSession, node chatdNode, timeout time.Duration) error {
	if session == nil || session.conn == nil {
		return fmt.Errorf("chatd session is not open")
	}
	deadline := time.Now().Add(textMessageSendTimeout(timeout))
	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
		deadline = ctxDeadline
	}
	if !deadline.After(time.Now()) {
		return fmt.Errorf("WA text message send timed out")
	}
	_ = session.conn.SetWriteDeadline(deadline)
	defer func() { _ = session.conn.SetWriteDeadline(time.Time{}) }()
	return session.transport.sendNode(node)
}

func textMessageSendTimeout(timeout time.Duration) time.Duration {
	if timeout <= 0 {
		return defaultTextMessageSendTimeout
	}
	return timeout
}

func chatdSendError(err error) error {
	message := "native chatd send failed"
	if ne, ok := err.(net.Error); ok && ne.Timeout() {
		message += ": timeout"
	} else if snippet := chatdSafeFailureMessage(err); snippet != "" {
		message += ": " + snippet
	}
	return NewError(waappv1.WaErrorCode_WA_ERROR_CODE_REJECTED, message, true)
}

func isChatdSendError(err error) bool {
	var appErr *AppError
	return errors.As(err, &appErr) && strings.HasPrefix(appErr.Message, "native chatd send failed")
}

func newTextProviderMessageID(clientID string) string {
	clientID = strings.TrimSpace(clientID)
	if validTextProviderMessageID(clientID) {
		return clientID
	}
	return "3EB0" + strings.ToUpper(hex.EncodeToString(randomBytes(8)))
}

func validTextProviderMessageID(value string) bool {
	if value == "" || len(value) > 96 {
		return false
	}
	for _, r := range value {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-' || r == '.' {
			continue
		}
		return false
	}
	return true
}
