package main

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	provisioningapp "f_ccg_video_back_ddd/internal/application/provisioning"
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

func TestRunAllResources(t *testing.T) {
	t.Parallel()

	tasks := []cli.Task{
		cli.TaskMigrateDB,
		cli.TaskVideoPartitions,
		cli.TaskBanner,
		cli.TaskESIndex,
		cli.TaskResourceSync,
		cli.TaskRBACInit,
		cli.TaskSuperUser,
	}
	for _, tt := range []struct {
		name    string
		failAt  cli.Task
		initErr bool
	}{
		{name: "all seven succeed"},
		{name: "migration failure stops before seeding", failAt: cli.TaskMigrateDB},
		{name: "initialization failure stops remaining tasks", failAt: cli.TaskBanner, initErr: true},
		{name: "execution failure cleans up and stops", failAt: cli.TaskBanner},
		{name: "rbac failure prevents super user creation", failAt: cli.TaskRBACInit},
		{name: "super user failure is returned", failAt: cli.TaskSuperUser},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			opts := &cli.Options{All: true, File: "custom.yaml", Args: []string{"test-admin", "test-password"}}
			var events, wantEvents []string
			wantCode := 0
			for _, task := range tasks {
				wantEvents = append(wantEvents, fmt.Sprintf("init %d", task))
				if task == tt.failAt && tt.initErr {
					wantCode = 1
					break
				}
				wantEvents = append(wantEvents, fmt.Sprintf("execute %d", task), fmt.Sprintf("cleanup %d", task))
				if task == tt.failAt {
					wantCode = 1
					break
				}
			}

			initializer := func(task cli.Task, gotOpts *cli.Options) (*cli.Executor, func(), error) {
				if gotOpts.File != opts.File || !reflect.DeepEqual(gotOpts.Args, opts.Args) {
					t.Fatal("all tasks must preserve config options and credentials")
				}
				events = append(events, fmt.Sprintf("init %d", task))
				if task == tt.failAt && tt.initErr {
					return nil, nil, errors.New("initialization failed")
				}
				executor := cli.NewMigrateDBExecutor(databaseMigratorFunc(func() error {
					events = append(events, fmt.Sprintf("execute %d", task))
					if task == tt.failAt {
						return errors.New("execution failed")
					}
					return nil
				}))
				return executor, func() {
					events = append(events, fmt.Sprintf("cleanup %d", task))
				}, nil
			}

			if got := runMaintenanceTaskWithInitializer(ctx, cli.ResolveTask(opts), opts, initializer); got != wantCode {
				t.Fatalf("exit code = %d, want %d", got, wantCode)
			}
			if !reflect.DeepEqual(events, wantEvents) {
				t.Fatalf("task events = %v, want %v", events, wantEvents)
			}
		})
	}
}

func TestRunAllSuperUserCredentials(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name     string
		args     []string
		wantArgs []string
		wantCode int
	}{
		{name: "default credentials", wantArgs: []string{"ccg", "123456"}},
		{name: "custom credentials", args: []string{"custom-admin", "test-password"}, wantArgs: []string{"custom-admin", "test-password"}},
		{name: "partial credentials fail before initialization", args: []string{"custom-admin"}, wantCode: 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			opts := &cli.Options{All: true, File: "config_dev.yaml", Args: tt.args}
			var gotArgs []string
			var initCalls int
			initializer := func(task cli.Task, gotOpts *cli.Options) (*cli.Executor, func(), error) {
				initCalls++
				if gotOpts.File != opts.File {
					t.Fatal("config path must be preserved")
				}
				if task == cli.TaskSuperUser {
					service := superUserProvisionerFunc(func(_ context.Context, args []string) (*provisioningapp.SuperUserResult, error) {
						gotArgs = append([]string(nil), args...)
						return &provisioningapp.SuperUserResult{Username: args[0]}, nil
					})
					return cli.NewSuperUserExecutor(service, nil), nil, nil
				}
				return cli.NewMigrateDBExecutor(databaseMigratorFunc(func() error { return nil })), nil, nil
			}

			if got := runMaintenanceTaskWithInitializer(context.Background(), cli.ResolveTask(opts), opts, initializer); got != tt.wantCode {
				t.Fatalf("exit code = %d, want %d", got, tt.wantCode)
			}
			if !reflect.DeepEqual(gotArgs, tt.wantArgs) {
				t.Fatal("super user credentials were not forwarded correctly")
			}
			if tt.wantCode != 0 && initCalls != 0 {
				t.Fatal("invalid credentials must fail before initializing resources")
			}
			if !reflect.DeepEqual(opts.Args, tt.args) {
				t.Fatal("default credentials must not mutate the caller's options")
			}
		})
	}
}

type superUserProvisionerFunc func(context.Context, []string) (*provisioningapp.SuperUserResult, error)

func (f superUserProvisionerFunc) Provision(ctx context.Context, args []string) (*provisioningapp.SuperUserResult, error) {
	return f(ctx, args)
}
