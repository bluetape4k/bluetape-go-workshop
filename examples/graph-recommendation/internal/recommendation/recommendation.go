package recommendation

import (
	"context"
	"sort"
)

// Recommend 는 한 번의 결정론적 실행으로 상품과 follow 후보를 함께 계산합니다.
func (g *Graph) Recommend(ctx context.Context, seedID string, limit int) (Report, error) {
	if err := g.validateRequest(ctx, seedID, limit); err != nil {
		return Report{}, err
	}
	products, err := g.RecommendProducts(ctx, seedID, limit)
	if err != nil {
		return Report{}, err
	}
	follows, err := g.RecommendFollows(ctx, seedID, limit)
	if err != nil {
		return Report{}, err
	}
	return Report{
		SeedUser:               seedID,
		ProductRecommendations: products,
		FollowRecommendations:  follows,
	}, nil
}

// RecommendProducts 는 2-hop PURCHASED evidence의 고유 공동구매자 수로 상품을 점수화합니다.
func (g *Graph) RecommendProducts(ctx context.Context, seedID string, limit int) ([]ProductRecommendation, error) {
	if g == nil {
		return nil, ErrInvalidGraph
	}
	if err := g.validateRequest(ctx, seedID, limit); err != nil {
		return nil, err
	}

	seedPurchased := make(map[string]struct{})
	candidateBuyers := make(map[string]map[string]string)
	for _, seedPurchase := range g.purchasesByUser[seedID] {
		if err := contextError(ctx); err != nil {
			return nil, err
		}
		seedPurchased[seedPurchase.productID] = struct{}{}
	}
	for _, seedPurchase := range g.purchasesByUser[seedID] {
		if err := contextError(ctx); err != nil {
			return nil, err
		}
		for _, coBuyerPurchase := range g.purchasesByProduct[seedPurchase.productID] {
			if err := contextError(ctx); err != nil {
				return nil, err
			}
			coBuyerID := coBuyerPurchase.userID
			if coBuyerID == seedID {
				continue
			}
			for _, candidatePurchase := range g.purchasesByUser[coBuyerID] {
				if err := contextError(ctx); err != nil {
					return nil, err
				}
				candidateID := candidatePurchase.productID
				if _, alreadyPurchased := seedPurchased[candidateID]; alreadyPurchased {
					continue
				}
				buyers := candidateBuyers[candidateID]
				if buyers == nil {
					buyers = make(map[string]string)
					candidateBuyers[candidateID] = buyers
				}
				if previous, exists := buyers[coBuyerID]; !exists || seedPurchase.productID < previous {
					buyers[coBuyerID] = seedPurchase.productID
				}
			}
		}
	}

	candidateIDs := make([]string, 0, len(candidateBuyers))
	for candidateID := range candidateBuyers {
		candidateIDs = append(candidateIDs, candidateID)
	}
	results := make([]ProductRecommendation, 0, len(candidateIDs))
	for _, candidateID := range candidateIDs {
		if err := contextError(ctx); err != nil {
			return nil, err
		}
		buyers := candidateBuyers[candidateID]
		coBuyerIDs := make([]string, 0, len(buyers))
		for coBuyerID := range buyers {
			coBuyerIDs = append(coBuyerIDs, coBuyerID)
		}
		sort.Strings(coBuyerIDs)
		evidence := make([]ProductEvidence, 0, len(coBuyerIDs))
		for _, coBuyerID := range coBuyerIDs {
			if err := contextError(ctx); err != nil {
				return nil, err
			}
			sharedProductID := buyers[coBuyerID]
			path, err := g.productEvidencePath(seedID, sharedProductID, coBuyerID, candidateID)
			if err != nil {
				return nil, ErrInvalidGraph
			}
			evidence = append(evidence, ProductEvidence{
				CoBuyerID:       coBuyerID,
				SharedProductID: sharedProductID,
				Path:            []string{seedID, sharedProductID, coBuyerID, candidateID},
				graphPath:       path,
			})
		}
		results = append(results, ProductRecommendation{
			ProductID: candidateID,
			Score:     len(coBuyerIDs),
			Evidence:  evidence,
		})
	}
	sort.Slice(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		return results[i].ProductID < results[j].ProductID
	})
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	return capProductResults(results, limit), nil
}

// RecommendFollows 는 seed의 직접 follow를 제외하고 FOAF 후보를 점수화합니다.
func (g *Graph) RecommendFollows(ctx context.Context, seedID string, limit int) ([]FollowRecommendation, error) {
	if g == nil {
		return nil, ErrInvalidGraph
	}
	if err := g.validateRequest(ctx, seedID, limit); err != nil {
		return nil, err
	}

	direct := make(map[string]struct{}, len(g.followsByUser[seedID]))
	candidateVia := make(map[string]map[string]struct{})
	for _, firstHop := range g.followsByUser[seedID] {
		if err := contextError(ctx); err != nil {
			return nil, err
		}
		direct[firstHop.followee] = struct{}{}
	}
	for _, firstHop := range g.followsByUser[seedID] {
		if err := contextError(ctx); err != nil {
			return nil, err
		}
		for _, secondHop := range g.followsByUser[firstHop.followee] {
			if err := contextError(ctx); err != nil {
				return nil, err
			}
			candidateID := secondHop.followee
			if candidateID == seedID {
				continue
			}
			if _, alreadyFollowed := direct[candidateID]; alreadyFollowed {
				continue
			}
			via := candidateVia[candidateID]
			if via == nil {
				via = make(map[string]struct{})
				candidateVia[candidateID] = via
			}
			via[firstHop.followee] = struct{}{}
		}
	}

	candidateIDs := make([]string, 0, len(candidateVia))
	for candidateID := range candidateVia {
		candidateIDs = append(candidateIDs, candidateID)
	}
	results := make([]FollowRecommendation, 0, len(candidateIDs))
	for _, candidateID := range candidateIDs {
		if err := contextError(ctx); err != nil {
			return nil, err
		}
		viaIDs := make([]string, 0, len(candidateVia[candidateID]))
		for viaID := range candidateVia[candidateID] {
			viaIDs = append(viaIDs, viaID)
		}
		sort.Strings(viaIDs)
		evidence := make([]FollowEvidence, 0, len(viaIDs))
		for _, viaID := range viaIDs {
			if err := contextError(ctx); err != nil {
				return nil, err
			}
			path, err := g.followEvidencePath(seedID, viaID, candidateID)
			if err != nil {
				return nil, ErrInvalidGraph
			}
			evidence = append(evidence, FollowEvidence{
				ViaUserID: viaID,
				Path:      []string{seedID, viaID, candidateID},
				graphPath: path,
			})
		}
		results = append(results, FollowRecommendation{
			UserID:   candidateID,
			Score:    len(viaIDs),
			Evidence: evidence,
		})
	}
	sort.Slice(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		return results[i].UserID < results[j].UserID
	})
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	return capFollowResults(results, limit), nil
}

func contextError(ctx context.Context) error {
	if ctx == nil {
		return ErrInvalidContext
	}
	return ctx.Err()
}

func capProductResults(results []ProductRecommendation, limit int) []ProductRecommendation {
	if limit == 0 || limit >= len(results) {
		return results
	}
	return results[:limit]
}

func capFollowResults(results []FollowRecommendation, limit int) []FollowRecommendation {
	if limit == 0 || limit >= len(results) {
		return results
	}
	return results[:limit]
}
