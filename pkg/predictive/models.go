package predictive

import (
    "math"
    
)

// ForecastRequest represents a prediction request
type ForecastRequest struct {
    NodeID      string                 `json:"node_id"`
    Property    string                 `json:"property"`
    Horizon     int64                  `json:"horizon"`      // microseconds
    Confidence  float64                `json:"confidence"`   // 0-1, default 0.95
    Parameters  map[string]interface{} `json:"parameters,omitempty"`
}

// ForecastResult represents a prediction result
type ForecastResult struct {
    PointEstimate float64                `json:"point_estimate"`
    LowerBound    float64                `json:"lower_bound"`
    UpperBound    float64                `json:"upper_bound"`
    Confidence    float64                `json:"confidence"`
    ModelUsed     string                 `json:"model_used"`
    Features      map[string]interface{} `json:"features,omitempty"`
}

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
        x := float64(i) // Use index as x for simplicity
        sumX += x
        sumY += y
        sumXY += x * y
        sumX2 += x * x
        sumY2 += y * y
    }
    
    slope := (n*sumXY - sumX*sumY) / (n*sumX2 - sumX*sumX)
    intercept := (sumY - slope*sumX) / n
    
    // Calculate R-squared
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

// Predict uses the trend to forecast future values
func (t *SimpleLinearTrend) Predict(steps int, confidence float64) (point, lower, upper float64) {
    x := float64(steps)
    point = t.Slope*x + t.Intercept
    
    // Simple confidence interval based on R-squared
    margin := math.Abs(point) * (1 - t.RSquared) * (1 - confidence)
    lower = point - margin
    upper = point + margin
    
    return
}

// MovingAverage calculates simple moving average forecast
func MovingAverage(values []float64, window int, steps int) float64 {
    if len(values) < window {
        window = len(values)
    }
    if window == 0 {
        return 0
    }
    
    sum := 0.0
    for i := len(values) - window; i < len(values); i++ {
        sum += values[i]
    }
    return sum / float64(window)
}

// ExponentialSmoothing performs simple exponential smoothing
func ExponentialSmoothing(values []float64, alpha float64, steps int) float64 {
    if len(values) == 0 {
        return 0
    }
    
    forecast := values[0]
    for i := 1; i < len(values); i++ {
        forecast = alpha*values[i] + (1-alpha)*forecast
    }
    
    // For multiple steps, keep the same forecast
    return forecast
}
