package stat

import (
	"fmt"
	"os"
	"testing"
)

func TestCheckEmail(t *testing.T) {
	ss := []string{
		// valid
		"test@example.com",
		"john.doe@sub.domain.co.uk",
		"user+tag@gmail.com",
		"user_123@my-website.io",
		"a@b.co",
		"no-reply@this-domain-does-not-exist.com",

		// invalid
		"plainaddress",
		"@missing-local-part.com",
		"user@.com",                 // start dot
		"user@domain-.com",          // domain end '-'
		"user@domain.c",             // TLD short
		"user@domain..com",          // dot/dot
		" leading.space@domain.com", // leading space
	}
	for _, s := range ss {
		if emailRegex.MatchString(s) {
			fmt.Fprintf(os.Stderr, "valid: %s\n", s)
			continue
		}
		fmt.Fprintf(os.Stderr, "invalid: %s\n", s)
	}
}

func TestSafePassword(t *testing.T) {
	ss := []string{
		"1", "hellow222", "jkac",
	}
	for _, s := range ss {
		fmt.Fprintf(os.Stderr, "%s\n", safePassword(s))
	}
}

func TestListConfig(t *testing.T) {
	vals, err := listConfig(t.Context(), "/tmp/jack")
	if err != nil {
		return
	}
	for k, v := range vals {
		fmt.Fprintf(os.Stderr, "%s = %s\n", k, v)
	}
	checkRemote(vals)
}

func TestTruncatedName(t *testing.T) {
	sss := []string{
		"cmd/hot/pkg/size/render.go",
		"Understand that enabling this registry setting will only affect applications that have been",
		"",
		"ProjectContractChargingPeriodProjectAccountReferenceVMFactoryBuilderStrategyDevOptsClassV2.md",
		"HasThisTypePatternTriedToSneakInSomeGenericOrParameterizedTypePatternMatchingStuffAnywhereVisitor",
		"doc/org.aspectj/aspectjweaver/1.8.10/org/aspectj/weaver/patterns/HasThisTypePatternTriedToSneakInSomeGenericOrParameterizedTypePatternMatchingStuffAnywhereVisitor.html",
		"doc/org.aspectj/aspectjweaver/1.8.10/org/aspectj/weaver/patterns/HasThisTypePatternTriedToSneakInSomeGenericOrParameterizedTypePatternMatching/StuffAnywhereVisitor.html",
	}
	for _, s := range sss {
		fmt.Fprintf(os.Stderr, "%s\n", truncatedName(s, 80))
	}
}
