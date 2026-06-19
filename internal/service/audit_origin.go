package service

const AuditSourceEmbed = "embed"

type AuditOrigin struct {
	Source           string
	EmbedAppID       string
	EmbedSessionID   uint64
	ExternalUserID   string
	ExternalUserName string
	SessionKey       string
}

func addAuditOriginPayload(payload map[string]any, origin AuditOrigin) {
	if origin.Source != "" {
		payload["source"] = origin.Source
	}
	if origin.EmbedAppID != "" {
		payload["app_id"] = origin.EmbedAppID
		payload["embed_app_id"] = origin.EmbedAppID
	}
	if origin.EmbedSessionID > 0 {
		payload["embed_session_id"] = origin.EmbedSessionID
	}
	if origin.ExternalUserID != "" {
		payload["external_user_id"] = origin.ExternalUserID
	}
	if origin.ExternalUserName != "" {
		payload["external_user_name"] = origin.ExternalUserName
	}
	if origin.SessionKey != "" {
		payload["session_key"] = origin.SessionKey
	}
}
