package main

import (
	"encoding/hex"
	"fmt"
	"io"
	"strings"

	"github.com/charkala/peth/internal/wallet"
)

// runWalletSign signs a message with the named wallet and prints the hex signature.
func runWalletSign(args []string, stdout io.Writer, makeKeystore func() (*wallet.Keystore, error)) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: peth wallet sign <wallet-name> <hex-or-utf8-message>")
	}
	name := args[0]
	msgInput := args[1]

	ks, err := makeKeystore()
	if err != nil {
		return err
	}
	key, err := ks.Get(name)
	if err != nil {
		return err
	}

	// Decode message: if 0x-prefixed hex, decode to string; otherwise use as-is
	var message string
	if strings.HasPrefix(msgInput, "0x") {
		b, err := hex.DecodeString(msgInput[2:])
		if err != nil {
			return fmt.Errorf("decode hex message: %w", err)
		}
		message = string(b)
	} else {
		message = msgInput
	}

	sig, err := wallet.PersonalSign(key.PrivateKey, message)
	if err != nil {
		return fmt.Errorf("sign: %w", err)
	}

	fmt.Fprintln(stdout, sig)
	return nil
}
