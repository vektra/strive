package strive

import "code.google.com/p/gogoprotobuf/proto"

func UnknownError(str string) *Error {
	return &Error{
		ErrorId:     Error_UNKNOWN,
		Description: proto.String(str),
	}
}

func UnknownMessage(str string) *Error {
	return &Error{
		ErrorId:     Error_UNKNOWN_MESSAGE,
		Description: proto.String(str),
	}
}

func UnknownTask(str string) *Error {
	return &Error{
		ErrorId:     Error_UNKNOWN_TASK,
		Description: proto.String(str),
	}
}
