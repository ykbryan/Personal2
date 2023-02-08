package service

import (
	"context"
	"math"
	"sort"
	"strconv"
	"strings"

	api_v1_entities_pb "github.com/tikivn/genproto/go/tiki/smart_api/v1/entities"
	common_entities "github.com/tikivn/genproto/go/tiki/smarter/common/entities"
	"github.com/tikivn/ko/pkg/logger"
	"github.com/tikivn/smarter/pkg/pagings"
	"github.com/tikivn/smarter/pkg/utils"
	"github.com/tikivn/smarter/pkg/utils/flags"
	"github.com/tikivn/smarter/services/smarter_api/repo/mocks"
)

type TikiNgonCartSuggestService struct {
	productSuggest ProductSuggestsService
	logger         logger.Logger
}

func NewTikiNgonCartSuggestService(
	productSuggest ProductSuggestsService,
) *TikiNgonCartSuggestService {
	return &TikiNgonCartSuggestService{
		productSuggest: productSuggest,
		logger:         utils.LoggerFactory("TikiNgonCartSuggestService"),
	}
}

type TikiNgonCartSuggestIn struct {
	Query          map[string]string
	BlockInfo      *BlockFullInfo
	ModelReq       *common_entities.ModelProductSuggest
	Limit          uint32
	Cursor         pagings.Cursor
	OrderValueGap  float64
	CartProductIds string
}

func (t *TikiNgonCartSuggestService) GetSuggestProducts(
	ctx context.Context,
	in *TikiNgonCartSuggestIn,
) (*ProductSuggestTransformOut, error) {
	productSuggestTransformOut, err := t.GetStoreProducts(ctx, in)
	if err != nil {
		return nil, err
	}
	listingProducts := productSuggestTransformOut.Products

	cartProductIdsMap := t.getCartProductIdsMap(in.CartProductIds)

	var qualifiedProducts []*api_v1_entities_pb.ListingsProductInfo
	for _, listingProduct := range listingProducts {
		if t.isQualified(in.OrderValueGap, cartProductIdsMap, listingProduct) {
			listingProduct.ProductRecoScore = t.getProductRecoScore(
				in.OrderValueGap,
				listingProduct,
			)

			qualifiedProducts = append(qualifiedProducts, listingProduct)
		}
	}
	sort.Slice(qualifiedProducts, func(i, j int) bool {
		return qualifiedProducts[i].ProductRecoScore < qualifiedProducts[j].ProductRecoScore
	})

	limit := math.Min(float64(in.Limit), float64(len(qualifiedProducts)))
	productSuggestTransformOut.Products = qualifiedProducts[:int(limit)-1]

	return productSuggestTransformOut, nil
}

var extractRulebaseLimit = 1000

func (t *TikiNgonCartSuggestService) GetStoreProducts(
	ctx context.Context,
	in *TikiNgonCartSuggestIn,
) (*ProductSuggestTransformOut, error) {
	if !flags.HasFlag("TIKINGON_CART_SUGGEST_MOCK") {
		return t.productSuggest.GetTransform(ctx, &ProductSuggestIn{
			BlockInfo:  in.BlockInfo,
			ModelReq:   in.ModelReq,
			Limit:      uint32(extractRulebaseLimit),
			Offset:     in.Cursor,
			QueryValue: in.Query,
			BlockCode:  in.BlockInfo.Block.Code,
		})
	}

	return &ProductSuggestTransformOut{
		Products: mocks.ListingProductsMock,
		ModelDebug: &common_entities.ModelDebug{
			O:  0,
			Ro: 0,
			Ao: 0,
		},
	}, nil
}

func (t *TikiNgonCartSuggestService) getProductRecoScore(
	orderValueGap float64,
	product *api_v1_entities_pb.ListingsProductInfo,
) float64 {
	priorityCateRank := t.getPriorityCateRank(product)
	productRecoScore := (product.Price - orderValueGap) + float64(priorityCateRank)

	return productRecoScore
}

func (t *TikiNgonCartSuggestService) isQualified(
	orderValueGap float64,
	cartProductIds map[string]int,
	product *api_v1_entities_pb.ListingsProductInfo,
) bool {
	if product.Price >= orderValueGap &&
		!t.isAlreadyInCart(cartProductIds, int(product.Id)) &&
		t.isInPriorityCates(product) {
		return true
	}

	return false
}

func (t *TikiNgonCartSuggestService) isAlreadyInCart(
	cartProductIds map[string]int,
	productID int,
) bool {
	_, ok := cartProductIds[strconv.Itoa(productID)]

	return ok
}

var (
	sortedSuggestCate = map[string]int{
		"54276": 1,
		"54302": 2,
		"44824": 3,
		"54330": 4,
		"54290": 5,
		"54344": 6,
		"54412": 7,
		"54384": 8,
		"54362": 9,
		"54500": 10,
		"54474": 11,
	}
	notPriorityCateRank = 100
)

func (t *TikiNgonCartSuggestService) isInPriorityCates(
	product *api_v1_entities_pb.ListingsProductInfo,
) bool {
	//nolint:gomnd
	cate2Id := GetCateID(product.PrimaryCategoryPath, 2)
	_, ok := sortedSuggestCate[cate2Id]

	return ok
}

func (t *TikiNgonCartSuggestService) getPriorityCateRank(
	product *api_v1_entities_pb.ListingsProductInfo,
) int {
	//nolint:gomnd
	cate2Id := GetCateID(product.PrimaryCategoryPath, 2)
	if priorityCateRank, ok := sortedSuggestCate[cate2Id]; ok {
		return priorityCateRank
	}

	return notPriorityCateRank
}

func GetCateID(
	primaryCategoryPath string,
	cateLevel int,
) string {
	cateTree := strings.Split(primaryCategoryPath, "/")

	return cateTree[cateLevel+1]
}

func (t *TikiNgonCartSuggestService) getCartProductIdsMap(
	cartProductIds string,
) map[string]int {
	cartProductIdsList := strings.Split(cartProductIds, ",")
	cartProductIdsMap := SliceToIntMap(cartProductIdsList)

	return cartProductIdsMap
}

func SliceToIntMap(elements []string) map[string]int {
	elementMap := make(map[string]int)
	for _, element := range elements {
		elementMap[element]++
	}

	return elementMap
}
