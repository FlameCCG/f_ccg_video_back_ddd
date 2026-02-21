package main

import (
	"context"
	"errors"
	"testing"

	"f_ccg_video_back_ddd/internal/interfaces/cli"
)

func TestRunMaintenanceTaskExitCodesAndCleanup(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		initializeErr error
		executeErr    error
		wantCode      int
		wantExecute   int
		wantCleanup   int
	}{
		{
			name:        "success exits zero",
			wantCode:    0,
			wantExecute: 1,
			wantCleanup: 1,
		},
		{
			name:        "task failure exits one",
			executeErr:  errors.New("task failed"),
			wantCode:    1,
			wantExecute: 1,
			wantCleanup: 1,
		},
		{
			name:          "initialization failure exits one",
			initializeErr: errors.New("initialization failed"),
			wantCode:      1,
			wantExecute:   0,
			wantCleanup:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var executeCalls, cleanupCalls int
			initializer := func(
				task cli.Task,
				opts *cli.Options,
			) (*cli.Executor, func(), error) {
				if task != cli.TaskMigrateDB {
					t.Fatalf("task = %v, want TaskMigrateDB", task)
				}
				if opts == nil || len(opts.Args) != 1 || opts.Args[0] != "arg" {
					t.Fatalf("options = %#v", opts)
				}
				if tt.initializeErr != nil {
					return nil, nil, tt.initializeErr
				}

				executor := cli.NewMigrateDBExecutor(databaseMigratorFunc(func() error {
					executeCalls++

					return tt.executeErr
				}))

				return executor, func() { cleanupCalls++ }, nil
			}

			got := runMaintenanceTaskWithInitializer(
				context.Background(),
				cli.TaskMigrateDB,
				&cli.Options{Args: []string{"arg"}},
				initializer,
			)
			if got != tt.wantCode {
				t.Fatalf("exit code = %d, want %d", got, tt.wantCode)
			}
			if executeCalls != tt.wantExecute {
				t.Fatalf("execute calls = %d, want %d", executeCalls, tt.wantExecute)
			}
			if cleanupCalls != tt.wantCleanup {
				t.Fatalf("cleanup calls = %d, want %d", cleanupCalls, tt.wantCleanup)
			}
		})
	}
}

type databaseMigratorFunc func() error

func (f databaseMigratorFunc) AutoMigrate() error {
	return f()
}
