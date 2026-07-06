package xanship

import "fmt"

const (
	BindPolicyKeep         = "keep"
	BindPolicyWarn         = "warn"
	BindPolicyFail         = "fail"
	BindPolicyCopyToVolume = "copy-to-volume"

	ExistingPolicyFail    = "fail"
	ExistingPolicyReuse   = "reuse"
	ExistingPolicyReplace = "replace"
)

func normalizeBindPolicy(policy string) (string, error) {
	if policy == "" {
		return BindPolicyKeep, nil
	}
	switch policy {
	case BindPolicyKeep, BindPolicyWarn, BindPolicyFail, BindPolicyCopyToVolume:
		return policy, nil
	default:
		return "", fmt.Errorf("unsupported bind policy %q", policy)
	}
}

func normalizeExistingPolicy(policy string) (string, error) {
	if policy == "" {
		return ExistingPolicyReuse, nil
	}
	switch policy {
	case ExistingPolicyFail, ExistingPolicyReuse, ExistingPolicyReplace:
		return policy, nil
	default:
		return "", fmt.Errorf("unsupported existing policy %q", policy)
	}
}
