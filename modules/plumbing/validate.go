package plumbing

import (
	"bytes"
	"fmt"
)

type ErrBadReferenceName struct {
	Name string
}

func (err ErrBadReferenceName) Error() string {
	return fmt.Sprintf("bad revision name: '%s'", err.Name)
}

func IsErrBadReferenceName(err error) bool {
	_, ok := err.(*ErrBadReferenceName)
	return ok
}

// https://github.com/git/git/blob/ae73b2c8f1da39c39335ee76a0f95857712c22a7/refs.c#L41-L290

var (
	// refnameDisposition table
	//
	// Here golang's logic is different from C's, golang's strings are not NULL-terminated, so byte(0) is a forbidden character.
	refnameDisposition = [256]byte{
		4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4,
		4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4,
		4, 0, 0, 0, 0, 0, 0, 0, 0, 0, 5, 0, 0, 0, 2, 1,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 4, 0, 0, 0, 0, 4,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 4, 4, 0, 4, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 3, 0, 0, 4, 4,
	}
)

/*
 * How to handle various characters in refnames:
 * 0: An acceptable character for refs
 * 1: End-of-component
 * 2: ., look for a preceding . to reject .. in refs
 * 3: {, look for a preceding @ to reject @{ in refs
 * 4: A bad character: ASCII control characters, and
 *    ":", "?", "[", "\", "^", "~", SP, or TAB
 * 5: *, reject unless REFNAME_REFSPEC_PATTERN is set
 */
func checkReferenceNameComponent(refname []byte) int {
	last := byte(0)
	var i int
	for ; i < len(refname); i++ {
		ch := refname[i] & 255
		disp := refnameDisposition[ch]
		switch disp {
		case 1:
			goto OUT // Do not use range, which causes extra processing for goto statements.
		case 2:
			if last == '.' {
				return -1
			}
		case 3:
			if last == '@' {
				return -1
			}
		case 4:
			return -1
		case 5:
			// we not use pattern mode
			return -1
		}
		last = ch
	}
OUT:
	if i == 0 {
		return 0
	}
	if refname[0] == '.' {
		return -1
	}
	if bytes.HasSuffix(refname, []byte(".lock")) {
		return -1
	}
	return i
}

/*
 * Try to read one refname component from the front of refname.
 * Return the length of the component found, or -1 if the component is
 * not legal.  It is legal if it is something reasonable to have under
 * ".zeta/refs/"; We do not like it if:
 *
 * - it begins with ".", or
 * - it has double dots "..", or
 * - it has ASCII control characters, or
 * - it has ":", "?", "[", "\", "^", "~", SP, or TAB anywhere, or
 * - it has "*" anywhere unless REFNAME_REFSPEC_PATTERN is set, or
 * - it ends with a "/", or
 * - it ends with ".lock", or
 * - it contains a "@{" portion
 *
 * When sanitized is not NULL, instead of rejecting the input refname
 * as an error, try to come up with a usable replacement for the input
 * refname in it.
 */
func ValidateReferenceName(refname []byte) bool {
	if bytes.Equal(refname, []byte("@")) {
		return false
	}
	var componentLen int
	for {
		/* We are at the start of a path component. */
		if componentLen = checkReferenceNameComponent(refname); componentLen <= 0 {
			return false
		}
		if len(refname) == componentLen {
			break
		}
		refname = refname[componentLen+1:]
	}
	return refname[componentLen-1] != '.'
}

// ValidateBranchName: creating branches starting with - is not supported
func ValidateBranchName(branch []byte) bool {
	if len(branch) == 0 || branch[0] == '-' {
		return false
	}
	return ValidateReferenceName(branch)
}

// ValidateTagName: creating tags starting with - is not supported
func ValidateTagName(tag []byte) bool {
	if len(tag) == 0 || tag[0] == '-' {
		return false
	}
	return ValidateReferenceName(tag)
}
