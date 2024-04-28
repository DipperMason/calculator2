package agent

import (
	"errors"

	"github.com/Knetic/govaluate"
)

// CalculatorAgent представляет агента калькулятора
type CalculatorAgent struct{}

// Calculate выполняет математические вычисления
func (a *CalculatorAgent) Calculate(expression string) (float64, error) {
	expr, err := govaluate.NewEvaluableExpression(expression)
	if err != nil {
		return 0, err
	}
	result, err := expr.Evaluate(nil)
	if err != nil {
		return 0, err
	}
	if resFloat, ok := result.(float64); ok {
		return resFloat, nil
	}
	return 0, errors.New("результат не является числом")
}
