package app

import "testing"

// TestCompareReleaseVersions locks down the comparison that decides which
// release every user's updater points at: numeric segments first, then the
// channel rank (alpha < beta < rc < stable), then the pre-release number.
func TestCompareReleaseVersions(t *testing.T) {
	cases := []struct {
		name    string
		current string
		curCh   string
		target  string
		tgtCh   string
		want    int
	}{
		{"older stable gets update", "1.29", "stable", "1.30", "stable", -1},
		{"same stable", "1.30", "stable", "1.30", "stable", 0},
		{"newer stable stays", "1.31", "stable", "1.30", "stable", 1},
		{"trailing zero segments equal", "1.30", "stable", "1.30.0", "stable", 0},
		{"padded numeric compare", "1.9", "stable", "1.10", "stable", -1},
		{"stable newer than rc on same numbers", "1.30", "stable", "1.30", "rc", 1},
		{"rc older than stable gets update", "1.30", "rc", "1.30", "stable", -1},
		{"beta older than rc", "1.30", "beta", "1.30", "rc", -1},
		{"alpha older than beta", "1.30", "alpha", "1.30", "beta", -1},
		{"rc number bumps within channel", "1.30", "rc", "1.30.0-rc.2", "rc", -1},
		{"target suffix in tag parses", "1.30", "stable", "1.31.0-beta.1", "beta", -1},
		{"same version same channel with pre", "1.30.0-rc.1", "rc", "1.30.0-rc.1", "rc", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := compareReleaseVersions(tc.current, tc.curCh, tc.target, tc.tgtCh)
			if got != tc.want {
				t.Fatalf("compareReleaseVersions(%q,%q,%q,%q) = %d, want %d",
					tc.current, tc.curCh, tc.target, tc.tgtCh, got, tc.want)
			}
		})
	}
}

// TestChannelRankOrdering pins the channel ordering the comparison relies on.
func TestChannelRankOrdering(t *testing.T) {
	if !(channelRank("alpha") < channelRank("beta") &&
		channelRank("beta") < channelRank("rc") &&
		channelRank("rc") < channelRank("stable")) {
		t.Fatal("channel rank must order alpha < beta < rc < stable")
	}
}
