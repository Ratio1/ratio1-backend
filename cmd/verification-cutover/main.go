package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"
)

func main() {
	var opts cutoverOptions
	var timeout time.Duration

	flag.BoolVar(&opts.Apply, "apply", false, "apply the cutover transaction; omitted means read-only dry-run")
	flag.Int64Var(&opts.ExpectedTotal, "expected-total", -1, "required with --apply")
	flag.Int64Var(&opts.ExpectedPreserved, "expected-preserved", -1, "required with --apply")
	flag.Int64Var(&opts.ExpectedReset, "expected-reset", -1, "required with --apply")
	flag.Int64Var(&opts.ExpectedResetUserInfos, "expected-reset-user-infos", -1, "required with --apply")
	flag.DurationVar(&timeout, "timeout", 30*time.Second, "overall database operation timeout")
	flag.Parse()

	if err := validateOptions(opts); err != nil {
		fmt.Fprintln(os.Stderr, "invalid arguments:", err)
		os.Exit(2)
	}

	databaseConfig, err := loadDatabaseConfigFromEnvironment()
	if err != nil {
		fmt.Fprintln(os.Stderr, "database configuration:", err)
		os.Exit(2)
	}

	db, err := openDatabase(databaseConfig)
	if err != nil {
		fmt.Fprintln(os.Stderr, "open database:", err)
		os.Exit(1)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err = runCutover(ctx, db, opts, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "verification cutover:", err)
		os.Exit(1)
	}
}
