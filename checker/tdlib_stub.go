//go:build !tdlib

package checker

import (
	"context"
	"fmt"
	"time"

	"github.com/belaytzev/tdmeter/config"
)

// TDLibChecker is a stub for builds without TDLib support.
// Build with -tags=tdlib to enable the real TDLib implementation.
type TDLibChecker struct{}

func NewTDLibChecker(apiID int32, apiHash string, dbPath string, timeout time.Duration) (*TDLibChecker, error) {
	return nil, fmt.Errorf("tdlib support not compiled; build with -tags=tdlib")
}

func (c *TDLibChecker) Check(ctx context.Context, p config.ProxyConfig) (float64, error) {
	return 0, fmt.Errorf("tdlib support not compiled; build with -tags=tdlib")
}

func (c *TDLibChecker) Close() error {
	return nil
}
