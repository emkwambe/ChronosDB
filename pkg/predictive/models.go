package predictive

import (
    "math"
)

// SimpleLinearTrend implements basic trend forecasting
type SimpleLinearTrend struct {
    Slope     float64
    Intercept float64
    RSquared  float64
}

// CalculateTrend computes linear trend from historical values
func CalculateTrend(values []float64, timestamps []int64) *SimpleLinearTrend {
    n := float64(len(values))
    if n < 2 {
        return &SimpleLinearTrend{Slope: 0, Intercept: 0, RSquared: 0}
    }
    
    var sumX, sumY, sumXY, sumX2, sumY2 float64
    
    for i, y := range values {
        x := float64(i)
        sumX += x
        sumY += y
        sumXY += x * y
        sumX2 += x * x
        sumY2 += y * y
    }
    
    slope := (n*sumXY - sumX*sumY) / (n*sumX2 - sumX*sumX)
    intercept := (sumY - slope*sumX) / n
    
    var ssRes, ssTot float64
    for i, y := range values {
        x := float64(i)
        predicted := slope*x + intercept
        ssRes += math.Pow(y-predicted, 2)
        ssTot += math.Pow(y-sumY/n, 2)
    }
    rSquared := 1 - (ssRes / ssTot)
    
    return &SimpleLinearTrend{
        Slope:     slope,
        Intercept: intercept,
        RSquared:  rSquared,
    }
}

func (t *SimpleLinearTrend) Predict(steps int, confidence float64) (point, lower, upper float64) {
    x := float64(steps)
    point = t.Slope*x + t.Intercept
    
    margin := math.Abs(point) * (1 - t.RSquared) * (1 - confidence)
    lower = point - margin
    upper = point + margin
    
    return
}

// HoltWinters implements exponential smoothing with trend
type HoltWinters struct {
    Alpha   float64
    Beta    float64
    Level   float64
    Trend   float64
    Season  []float64
    Period  int
}

func NewHoltWinters(alpha, beta float64, period int) *HoltWinters {
    return &HoltWinters{
        Alpha:  alpha,
        Beta:   beta,
        Period: period,
        Season: make([]float64, period),
    }
}

func (hw *HoltWinters) Fit(values []float64) {
    if len(values) < 2 {
        return
    }
    
    hw.Level = values[0]
    hw.Trend = values[1] - values[0]
    
    if hw.Period > 0 && len(values) >= hw.Period {
        for i := 0; i < hw.Period && i < len(values); i++ {
            if hw.Level != 0 {
                hw.Season[i] = values[i] / hw.Level
            }
        }
    }
    
    for t := 1; t < len(values); t++ {
        oldLevel := hw.Level
        hw.Level = hw.Alpha*values[t] + (1-hw.Alpha)*(hw.Level+hw.Trend)
        hw.Trend = hw.Beta*(hw.Level-oldLevel) + (1-hw.Beta)*hw.Trend
        
        if hw.Period > 0 && t < len(values) {
            seasonIdx := t % hw.Period
            if hw.Level != 0 {
                hw.Season[seasonIdx] = hw.Alpha*(values[t]/hw.Level) + (1-hw.Alpha)*hw.Season[seasonIdx]
            }
        }
    }
}

func (hw *HoltWinters) Predict(steps int) float64 {
    forecast := hw.Level + float64(steps)*hw.Trend
    
    if hw.Period > 0 {
        seasonIdx := steps % hw.Period
        if seasonIdx < len(hw.Season) {
            forecast *= hw.Season[seasonIdx]
        }
    }
    
    return forecast
}

// ARIMAModel simplified implementation
type ARIMAModel struct {
    p            int
    d            int
    q            int
    coefficients []float64
    residuals    []float64
}

func NewARIMAModel(p, d, q int) *ARIMAModel {
    return &ARIMAModel{
        p:            p,
        d:            d,
        q:            q,
        coefficients: make([]float64, p+q),
    }
}

func (arima *ARIMAModel) Fit(values []float64) {
    if len(values) < arima.p+arima.q {
        return
    }
    
    diffed := make([]float64, len(values)-arima.d)
    copy(diffed, values)
    for i := 0; i < arima.d; i++ {
        for j := len(diffed) - 1; j > 0; j-- {
            diffed[j] = diffed[j] - diffed[j-1]
        }
        diffed = diffed[1:]
    }
    
    if arima.p > 0 && len(diffed) > arima.p {
        autoCorr := make([]float64, arima.p)
        meanVal := mean(diffed)
        
        for lag := 1; lag <= arima.p; lag++ {
            var num, den float64
            for i := lag; i < len(diffed); i++ {
                num += (diffed[i] - meanVal) * (diffed[i-lag] - meanVal)
                den += math.Pow(diffed[i-lag]-meanVal, 2)
            }
            if den != 0 {
                autoCorr[lag-1] = num / den
            }
        }
        
        for i := 0; i < arima.p && i < len(arima.coefficients); i++ {
            arima.coefficients[i] = autoCorr[i]
        }
    }
}

func (arima *ARIMAModel) Predict(steps int) float64 {
    if len(arima.coefficients) == 0 {
        return 0
    }
    
    forecast := 0.0
    for i := 0; i < arima.p && i < len(arima.coefficients); i++ {
        forecast += arima.coefficients[i]
    }
    
    return forecast
}

// EnsembleModel combines multiple models
type EnsembleModel struct {
    models     []interface{}
    weights    []float64
    modelNames []string
}

func NewEnsembleModel() *EnsembleModel {
    return &EnsembleModel{
        models:     make([]interface{}, 0),
        weights:    make([]float64, 0),
        modelNames: make([]string, 0),
    }
}

func (e *EnsembleModel) AddModel(name string, model interface{}, weight float64) {
    e.models = append(e.models, model)
    e.weights = append(e.weights, weight)
    e.modelNames = append(e.modelNames, name)
}

func (e *EnsembleModel) Predict(steps int) float64 {
    if len(e.models) == 0 {
        return 0
    }
    
    total := 0.0
    weightSum := 0.0
    
    for i, model := range e.models {
        var pred float64
        switch m := model.(type) {
        case *SimpleLinearTrend:
            p, _, _ := m.Predict(steps, 0.95)
            pred = p
        case *HoltWinters:
            pred = m.Predict(steps)
        case *ARIMAModel:
            pred = m.Predict(steps)
        default:
            continue
        }
        total += pred * e.weights[i]
        weightSum += e.weights[i]
    }
    
    if weightSum > 0 {
        return total / weightSum
    }
    return 0
}

func mean(values []float64) float64 {
    if len(values) == 0 {
        return 0
    }
    sum := 0.0
    for _, v := range values {
        sum += v
    }
    return sum / float64(len(values))
}
