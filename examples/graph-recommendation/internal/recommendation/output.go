package recommendation

import (
	"encoding/json"
)

// EncodeReport 는 문서와 CLI가 공유하는 trailing newline 포함 JSON을 반환합니다.
func EncodeReport(report Report) ([]byte, error) {
	normalized := report
	if normalized.ProductRecommendations == nil {
		normalized.ProductRecommendations = []ProductRecommendation{}
	}
	if normalized.FollowRecommendations == nil {
		normalized.FollowRecommendations = []FollowRecommendation{}
	}
	for index := range normalized.ProductRecommendations {
		if normalized.ProductRecommendations[index].Evidence == nil {
			normalized.ProductRecommendations[index].Evidence = []ProductEvidence{}
		}
	}
	for index := range normalized.FollowRecommendations {
		if normalized.FollowRecommendations[index].Evidence == nil {
			normalized.FollowRecommendations[index].Evidence = []FollowEvidence{}
		}
	}
	encoded, err := json.MarshalIndent(normalized, "", "  ")
	if err != nil {
		return nil, err
	}
	encoded = append(encoded, '\n')
	return encoded, nil
}
