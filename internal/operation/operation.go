package op

var (
	_           Operand
	OpenParen   = openParen{}
	ClosedParen = closeParen{}
	Add         = add{}
	Sub         = sub{}
	Mult        = mult{}
	Div         = div{}
)

var Operands = []Operand{OpenParen, ClosedParen, Add, Sub, Mult, Div}

var OperationPriority = map[Operand]int{
	OpenParen:   0,
	ClosedParen: 0,
	Add:         1,
	Sub:         1,
	Mult:        2,
	Div:         2,
}

type BinaryOperationInfo struct {
	A  float32 `json:"a"`
	B  float32 `json:"b"`
	Op string  `json:"op"`
}

func HaveOperand(symbol string) bool {
	for _, o := range Operands {
		if o.Symbol() == symbol {
			return true
		}
	}
	return false
}
