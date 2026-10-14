// Copyright 2025-2026 Docker, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"sync"
	"sync/atomic"

	"github.com/docker/secrets-engine/store"
	"github.com/docker/secrets-engine/store/keychain"
)

type syntheticSecret struct{}

func (*syntheticSecret) Marshal() ([]byte, error) {
	return []byte("synthetic-keyring-reproducer-value"), nil
}

func (*syntheticSecret) Unmarshal(value []byte) error {
	if string(value) != "synthetic-keyring-reproducer-value" {
		return errors.New("synthetic secret did not round-trip")
	}
	return nil
}

func (*syntheticSecret) Metadata() map[string]string {
	return map[string]string{"purpose": "keyring-crash-reproducer"}
}

func (*syntheticSecret) SetMetadata(map[string]string) error { return nil }

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	workers := flag.Int("workers", 8, "concurrent clients (use 1 for a sequential control)")
	iterations := flag.Int("iterations", 1500, "Save/Get/Delete cycles per client")
	flag.Parse()
	if *workers < 1 || *iterations < 1 || flag.NArg() != 0 {
		return errors.New("workers and iterations must be positive, with no positional arguments")
	}
	if os.Getenv("KEYCHAIN_REPRO_ISOLATED") != "1" {
		return errors.New("run with make keychain-gnome-repro: this probe can crash the Secret Service daemon")
	}

	ctx := context.Background()
	credentials, err := keychain.New(ctx, "keyring-reproducer", "synthetic", func(context.Context, store.ID) *syntheticSecret {
		return &syntheticSecret{}
	})
	if err != nil {
		return err
	}

	fmt.Printf("workers=%d iterations=%d\n", *workers, *iterations)
	var completed atomic.Int64
	var failed atomic.Bool
	var workersDone sync.WaitGroup
	failures := make(chan error, *workers)
	start := make(chan struct{})
	for worker := range *workers {
		workersDone.Go(func() {
			<-start
			id := store.MustParseID(fmt.Sprintf("probe-%d", worker))
			for iteration := range *iterations {
				if failed.Load() {
					return
				}
				if err := cycle(ctx, credentials, id); err != nil {
					failed.Store(true)
					failures <- fmt.Errorf("worker=%d iteration=%d: %w", worker, iteration, err)
					return
				}
				if count := completed.Add(1); count%500 == 0 {
					fmt.Printf("completed_cycles=%d\n", count)
				}
			}
		})
	}
	close(start)
	workersDone.Wait()
	close(failures)
	fmt.Printf("total_completed_cycles=%d\n", completed.Load())
	var allErrors []error
	for err := range failures {
		allErrors = append(allErrors, err)
	}
	if err := errors.Join(allErrors...); err != nil {
		return err
	}
	fmt.Println("No crash observed in this run; this does not establish that the race is fixed.")
	return nil
}

func cycle(ctx context.Context, credentials store.Store, id store.ID) error {
	if err := credentials.Save(ctx, id, &syntheticSecret{}); err != nil {
		return fmt.Errorf("save: %w", err)
	}
	if _, err := credentials.Get(ctx, id); err != nil {
		return fmt.Errorf("get: %w", err)
	}
	if err := credentials.Delete(ctx, id); err != nil {
		return fmt.Errorf("delete: %w", err)
	}
	return nil
}
