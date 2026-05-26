package grpcutil

import (
	"context"
	"errors"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
)

func Ping(ctx context.Context, conn *grpc.ClientConn) error {
	if conn == nil {
		return errors.New("grpc connection is nil")
	}

	conn.Connect()
	for {
		state := conn.GetState()
		switch state {
		case connectivity.Ready:
			return nil
		case connectivity.Shutdown:
			return errors.New("grpc connection is shut down")
		}

		if !conn.WaitForStateChange(ctx, state) {
			if err := ctx.Err(); err != nil {
				return fmt.Errorf("grpc connection did not become ready: %w", err)
			}
			return errors.New("grpc connection did not become ready")
		}
	}
}
