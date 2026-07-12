package oauth

type OAErrorCode = string

// https://datatracker.ietf.org/doc/html/rfc6749#section-4.1.2.1
const (
	invalidRequest          OAErrorCode = "invalid_request"
	unauthorizedClient      OAErrorCode = "unauthorized_client"
	accessDenied            OAErrorCode = "access_denied"
	unsupportedResponseType OAErrorCode = "unsupported_response_type"
	invalidScope            OAErrorCode = "invalid_scope"
	serverError             OAErrorCode = "server_error"
	temporarilyUnavailable  OAErrorCode = "temporarily_unavailable"
)
