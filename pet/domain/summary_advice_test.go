package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildAdviceExposesStructuredFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		kind       ListingIssueKind
		wantAction AdviceAction
	}{
		{name: "no photo", kind: IssueNoPhoto, wantAction: AdviceAddPhoto},
		{name: "no price", kind: IssueNoPrice, wantAction: AdviceSetPrice},
		{name: "short text", kind: IssueShortText, wantAction: AdviceExpandText},
		{name: "stale", kind: IssueStale, wantAction: AdviceRefreshListing},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			itemID := "abc123def456"
			facts := DayFacts{
				Issues: []ListingIssue{{ItemID: itemID, Title: "Велосипед", Kind: tc.kind}},
			}

			advice := BuildAdvice(facts)

			require.NotNil(t, advice)
			assert.Equal(t, tc.wantAction, advice.Action)
			assert.Equal(t, "Велосипед", advice.ItemTitle)
			require.NotNil(t, advice.ItemID)
			assert.Equal(t, itemID, *advice.ItemID)
		})
	}
}

func TestBuildAdviceWithoutIssues(t *testing.T) {
	t.Parallel()

	advice := BuildAdvice(DayFacts{})

	require.NotNil(t, advice)
	assert.Equal(t, AdviceCheckIn, advice.Action)
	assert.Empty(t, advice.ItemTitle)
	assert.Nil(t, advice.ItemID)
}
