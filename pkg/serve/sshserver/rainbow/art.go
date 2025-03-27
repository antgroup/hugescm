// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package rainbow

import (
	"fmt"
	"io"
	"math/rand/v2"
	"strings"
)

// ######## ######## ########    ###        ######   ######  ##     ##
//      ##  ##          ##      ## ##      ##    ## ##    ## ###   ###
//     ##   ##          ##     ##   ##     ##       ##       #### ####
//    ##    ######      ##    ##     ##     ######  ##       ## ### ##
//   ##     ##          ##    #########          ## ##       ##     ##
//  ##      ##          ##    ##     ##    ##    ## ##    ## ##     ##
// ######## ########    ##    ##     ##     ######   ######  ##     ##

const (
	// https://budavariam.github.io/asciiart-text/multi
	// Banner3
	zetaArt = `
'########:'########:'########::::'###::::::::::
..... ##:: ##.....::... ##..::::'## ##:::::::::
:::: ##::: ##:::::::::: ##:::::'##:. ##::::::::
::: ##:::: ######:::::: ##::::'##:::. ##:::::::
:: ##::::: ##...::::::: ##:::: #########:::::::
: ##:::::: ##:::::::::: ##:::: ##.... ##:::::::
 ########: ########:::: ##:::: ##:::: ##:::::::
........::........:::::..:::::..:::::..::::::::
`
	template = "Hi \x1b[38;2;67;233;123m%v\x1b[0m You've successfully authenticated, " +
		"but \x1b[38;2;72;198;239mZETA\x1b[0m does not provide shell access.\n" +
		"你好 \x1b[38;2;67;233;123m%v\x1b[0m 你已经成功通过身份验证，" +
		"但是 \x1b[38;2;72;198;239mZETA\x1b[0m 不提供 shell 访问。\n" +
		"使用签名（signing using）\x1b[38;2;177;244;207m%s\x1b[0m \x1b[38;2;250;112;154m%s\x1b[0m\n"
)

type DisplayOpts struct {
	UserName    string
	Fingerprint string
	KeyType     string
	Width       int // -1 not tty
}

func Display(w io.Writer, opts *DisplayOpts) {
	_, _ = w.Write([]byte("Welcome to ZETA 🎉🎉🎉\n"))
	if opts.Width >= 80 {
		rw := Light{
			Reader: strings.NewReader(zetaArt),
			Writer: w,
			Seed:   rand.Int64N(256),
		}
		_ = rw.Paint()
	}
	_, _ = fmt.Fprintf(w, template, opts.UserName, opts.UserName, opts.KeyType, opts.Fingerprint)
}
