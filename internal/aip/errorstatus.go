package aip

// ErrorStatus 对应谷歌 aip 中的 Status.code 参考 https://google.aip.dev/193#statuscode
// 参考 https://github.com/googleapis/googleapis/blob/master/google/rpc/code.proto.

type ErrorStatus int

const (
	StatusOK ErrorStatus = iota
	StatusCancelled
	StatusUnknown
	StatusInvalidArgument
	StatusDeadlineExceeded
	StatusNotFound
	StatusAlreadyExists
	StatusPermissionDenied
	StatusResourceExhausted
	StatusFailedPrecondition
	StatusAborted
	StatusOutOfRange
	StatusUnimplemented
	StatusInternal
	StatusUnavailable
	StatusDataLoss
	StatusUnauthenticated
)

var statusText = map[ErrorStatus]string{
	StatusOK:                 "OK",
	StatusCancelled:          "CANCELLED",
	StatusUnknown:            "UNKNOWN",
	StatusInvalidArgument:    "INVALID_ARGUMENT",
	StatusDeadlineExceeded:   "DEADLINE_EXCEEDED",
	StatusNotFound:           "NOT_FOUND",
	StatusAlreadyExists:      "ALREADY_EXISTS",
	StatusPermissionDenied:   "PERMISSION_DENIED",
	StatusResourceExhausted:  "RESOURCE_EXHAUSTED",
	StatusFailedPrecondition: "FAILED_PRECONDITION",
	StatusAborted:            "ABORTED",
	StatusOutOfRange:         "OUT_OF_RANGE",
	StatusUnimplemented:      "UNIMPLEMENTED",
	StatusInternal:           "INTERNAL",
	StatusUnavailable:        "UNAVAILABLE",
	StatusDataLoss:           "DATA_LOSS",
	StatusUnauthenticated:    "UNAUTHENTICATED",
}

var statusHTTPCode = map[ErrorStatus]int{
	StatusOK:                 200,
	StatusCancelled:          499,
	StatusUnknown:            500,
	StatusInvalidArgument:    400,
	StatusDeadlineExceeded:   504,
	StatusNotFound:           404,
	StatusAlreadyExists:      409,
	StatusPermissionDenied:   403,
	StatusResourceExhausted:  429,
	StatusFailedPrecondition: 400,
	StatusAborted:            409,
	StatusOutOfRange:         400,
	StatusUnimplemented:      501,
	StatusInternal:           500,
	StatusUnavailable:        503,
	StatusDataLoss:           500,
	StatusUnauthenticated:    401,
}

func (s ErrorStatus) String() string {
	return statusText[s]
}

func (s ErrorStatus) HTTPCode() int {
	return statusHTTPCode[s]
}
