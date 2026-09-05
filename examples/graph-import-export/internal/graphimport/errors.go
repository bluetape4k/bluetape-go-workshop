package graphimport

import "errors"

var (
	// ErrInvalidContext 는 import/export 호출에 context가 없음을 나타낸다.
	ErrInvalidContext = errors.New("graph import invalid context")
	// ErrInvalidPartner 는 named partner 식별자가 비어 있거나 잘못되었음을 나타낸다.
	ErrInvalidPartner = errors.New("graph import invalid partner")
	// ErrInvalidFormat 는 지원하지 않는 interchange format을 나타낸다.
	ErrInvalidFormat = errors.New("graph import invalid format")
	// ErrInvalidInput 는 reader 또는 writer 같은 입출력 경계가 잘못되었음을 나타낸다.
	ErrInvalidInput = errors.New("graph import invalid input")
	// ErrInvalidGraph 는 domain graph invariant를 위반한 record set을 나타낸다.
	ErrInvalidGraph = errors.New("graph import invalid graph")
	// ErrDuplicateRecord 는 vertex와 edge를 합친 record ID가 중복되었음을 나타낸다.
	ErrDuplicateRecord = errors.New("graph import duplicate record")
	// ErrUnsupportedRecord 는 이 예제가 약속한 account/device subset 밖의 record를 나타낸다.
	ErrUnsupportedRecord = errors.New("graph import unsupported record")
	// ErrNonScalarProperty 는 interchange subset 밖의 map/array/null property를 나타낸다.
	ErrNonScalarProperty = errors.New("graph import non-scalar property")
	// ErrInputTooLarge 는 caller가 설정한 전체 입력 byte 상한을 넘었음을 나타낸다.
	ErrInputTooLarge = errors.New("graph import input too large")
	// ErrSnapshotMismatch 는 두 format의 normalized snapshot이 다름을 나타낸다.
	ErrSnapshotMismatch = errors.New("graph import snapshot mismatch")
)
