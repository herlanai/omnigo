// Package startup implements the private updater-to-application readiness handshake.
package startup

import (
	"context"
	"fmt"
	"net"
	"os"
	"time"
)

const AddressEnv = "OMNIGO_UPDATE_READY_ADDRESS"
const TokenEnv = "OMNIGO_UPDATE_READY_TOKEN"

// Confirm reports native-host readiness to the update supervisor, if present.
// Credentials are consumed once and never passed to subsequent child processes.
func Confirm(ctx context.Context) error {
	address, token := os.Getenv(AddressEnv), os.Getenv(TokenEnv)
	if address == "" && token == "" {
		return nil
	}
	os.Unsetenv(AddressEnv)
	os.Unsetenv(TokenEnv)
	host, _, err := net.SplitHostPort(address)
	if err != nil || host != "127.0.0.1" || len(token) != 64 {
		return fmt.Errorf("startup: invalid confirmation credentials")
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	c, err := (&net.Dialer{}).DialContext(ctx, "tcp4", address)
	if err != nil {
		return err
	}
	defer c.Close()
	deadline, _ := ctx.Deadline()
	c.SetDeadline(deadline)
	_, err = fmt.Fprintln(c, token)
	return err
}
