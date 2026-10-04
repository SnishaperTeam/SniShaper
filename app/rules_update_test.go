package app

import "testing"

func TestRulesBlobSHAEmptyContent(t *testing.T) {
	// git 对空文件的 blob sha 是固定值
	if got := rulesBlobSHA(nil); got != "e69de29bb2d1d6434b8b29ae775ad8c2e48c5391" {
		t.Fatalf("unexpected blob sha for empty content: %s", got)
	}
}

func TestRulesBlobSHAKnownContent(t *testing.T) {
	// 与 `printf 'hello\n' | git hash-object --stdin` 输出一致
	if got := rulesBlobSHA([]byte("hello\n")); got != "ce013625030ba8dba906f756967f9e9ca394464a" {
		t.Fatalf("unexpected blob sha for 'hello\\n': %s", got)
	}
}
