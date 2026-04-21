package main

import (
	"fmt"
	"sort"

	"github.com/cushycush/store/v2/internal/config"
	"github.com/cushycush/store/v2/internal/secrets"
	"github.com/cushycush/store/v2/internal/ui"
	"github.com/spf13/cobra"
)

func runSecretSet(cmd *cobra.Command, args []string) error {
	_ = cmd

	root, err := config.FindRoot()
	if err != nil {
		return err
	}

	value := ""
	if len(args) == 2 {
		value = args[1]
		fmt.Fprintln(cmd.ErrOrStderr(), ui.Dim("  warning: passing a secret value as an argument may be logged by your shell history; prefer interactive input"))
	} else {
		value, err = promptHiddenValue("Enter secret value: ", "failed to read secret value")
		if err != nil {
			return err
		}
	}

	passphrase, err := getPassphrase()
	if err != nil {
		return err
	}

	secretMap, err := secrets.Load(root, passphrase)
	if err != nil {
		return err
	}
	secretMap[args[0]] = value

	if err := secrets.Save(root, passphrase, secretMap); err != nil {
		return err
	}

	fmt.Printf("%s %s\n", ui.Green("Set secret"), ui.Bold(args[0]))
	return nil
}

func runSecretGet(cmd *cobra.Command, args []string) error {
	_ = cmd

	root, err := config.FindRoot()
	if err != nil {
		return err
	}

	passphrase, err := getPassphrase()
	if err != nil {
		return err
	}

	secretMap, err := secrets.Load(root, passphrase)
	if err != nil {
		return err
	}

	value, ok := secretMap[args[0]]
	if !ok {
		return fmt.Errorf("secret %q not found", args[0])
	}

	fmt.Println(ui.FileName(value))
	return nil
}

func runSecretRemove(cmd *cobra.Command, args []string) error {
	_ = cmd

	root, err := config.FindRoot()
	if err != nil {
		return err
	}

	passphrase, err := getPassphrase()
	if err != nil {
		return err
	}

	secretMap, err := secrets.Load(root, passphrase)
	if err != nil {
		return err
	}

	if _, ok := secretMap[args[0]]; !ok {
		return fmt.Errorf("secret %q not found", args[0])
	}
	delete(secretMap, args[0])

	if err := secrets.Save(root, passphrase, secretMap); err != nil {
		return err
	}

	fmt.Printf("%s %s\n", ui.Green("Removed secret"), ui.Bold(args[0]))
	return nil
}

func runSecretList(cmd *cobra.Command, args []string) error {
	_ = cmd
	_ = args

	root, err := config.FindRoot()
	if err != nil {
		return err
	}

	passphrase, err := getPassphrase()
	if err != nil {
		return err
	}

	secretMap, err := secrets.Load(root, passphrase)
	if err != nil {
		return err
	}

	names := make([]string, 0, len(secretMap))
	for name := range secretMap {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		fmt.Println(ui.Bold(name))
	}

	return nil
}
