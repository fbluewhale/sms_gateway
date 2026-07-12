package sms

type LineType string

const (
	LineTypeExpress LineType = "express"
	LineTypeNormal  LineType = "normal"
)

func (lt LineType) IsValid() bool {
	return lt == LineTypeExpress || lt == LineTypeNormal
}

func (lt LineType) String() string {
	return string(lt)
}

type Destination string

func (d Destination) String() string {
	return string(d)
}

func (d Destination) IsValid() bool {
	return len(d) > 0
}
