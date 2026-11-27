package notify

import "errors"

var (
	ErrRecipientRejected = errors.New("recipient_rejected")
	ErrSendFailed        = errors.New("send_failed")
	ErrSendTimeout       = errors.New("send_timeout")
)

func ErrorCode(err error) string {
	switch {
	case errors.Is(err, ErrRecipientRejected):
		return "recipient_rejected"
	case errors.Is(err, ErrSendTimeout):
		return "send_timeout"
	case errors.Is(err, ErrSendFailed):
		return "send_failed"
	default:
		return "send_failed"
	}
}
