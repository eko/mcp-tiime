package tools

import (
	"context"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"mcp-tiime/internal/tiime"
)

// Auth runs the Tiime login, including the MFA verification step.
type Auth interface {
	StartLogin(
		ctx context.Context,
		method string,
	) (*tiime.LoginChallenge, error)
	CompleteLogin(
		ctx context.Context,
		code string,
	) (*tiime.LoginChallenge, error)
}

// StartLoginInput selects the MFA method.
type StartLoginInput struct {
	Method string `json:"method,omitempty" jsonschema:"MFA verification method: email (default), sms or otp"`
}

// CompleteLoginInput carries the verification code.
type CompleteLoginInput struct {
	Code string `json:"code" jsonschema:"Verification code received from Tiime"`
}

// LoginOutput reports the state of the login.
type LoginOutput struct {
	// Status is "logged_in" or "code_required".
	Status  string `json:"status"`
	Method  string `json:"method,omitempty"`
	Message string `json:"message,omitempty"`
	SentAt  string `json:"sent_at,omitempty"`
	Next    string `json:"next"`
}

// loginHandler implements the login tools, shared by the client and expert modes.
type loginHandler struct {
	auth Auth
}

func registerAuth(
	server *mcp.Server,
	auth Auth,
) {
	h := &loginHandler{auth: auth}

	mcp.AddTool(server, &mcp.Tool{
		Name: "start_login",
		Description: "Log in to Tiime with the credentials configured on the server. Call it when another Tiime tool fails " +
			"because the session is missing or expired. When MFA is enabled, Tiime sends a verification code " +
			"(by email by default): read it from the mailbox, in the Tiime message received after sent_at, " +
			"then call complete_login with it.",
	}, h.StartLogin)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "complete_login",
		Description: "Submit the Tiime MFA verification code requested by start_login. On a wrong code, a new challenge is returned and the call can be retried.",
	}, h.CompleteLogin)
}

// StartLogin implements the start_login tool.
func (h *loginHandler) StartLogin(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in StartLoginInput,
) (*mcp.CallToolResult, LoginOutput, error) {
	challenge, err := h.auth.StartLogin(ctx, in.Method)
	if err != nil {
		return nil, LoginOutput{}, err
	}

	return nil, loginOutput(challenge), nil
}

// CompleteLogin implements the complete_login tool.
func (h *loginHandler) CompleteLogin(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	in CompleteLoginInput,
) (*mcp.CallToolResult, LoginOutput, error) {
	challenge, err := h.auth.CompleteLogin(ctx, in.Code)
	if err != nil {
		return nil, LoginOutput{}, err
	}

	return nil, loginOutput(challenge), nil
}

func loginOutput(
	challenge *tiime.LoginChallenge,
) LoginOutput {
	if challenge == nil {
		return LoginOutput{
			Status: "logged_in",
			Next:   "The session is cached: retry the Tiime tool that failed.",
		}
	}

	return LoginOutput{
		Status:  "code_required",
		Method:  challenge.Method,
		Message: challenge.Message,
		SentAt:  challenge.SentAt.UTC().Format(time.RFC3339),
		Next:    "Get the verification code sent by Tiime after sent_at, then call complete_login with it.",
	}
}
