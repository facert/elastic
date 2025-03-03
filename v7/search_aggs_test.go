// Copyright 2012-present Oliver Eilhard. All rights reserved.
// Use of this source code is governed by a MIT-license.
// See http://olivere.mit-license.org/license.txt for details.

package elastic

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"
)

// TestAggs is an integration test for most aggregation types.
func TestAggs(t *testing.T) {
	client := setupTestClientAndCreateIndex(t) //, SetTraceLog(log.New(os.Stdout, "", log.LstdFlags)))

	tweet1 := tweet{
		User:     "olivere",
		Retweets: 108,
		Message:  "Welcome to Golang and Elasticsearch.",
		Image:    "http://golang.org/doc/gopher/gophercolor.png",
		Tags:     []string{"golang", "elasticsearch"},
		Location: "48.1333,11.5667", // lat,lon
		Created:  time.Date(2012, 12, 12, 17, 38, 34, 0, time.UTC),
	}
	tweet2 := tweet{
		User:     "olivere",
		Retweets: 0,
		Message:  "Another unrelated topic.",
		Tags:     []string{"golang"},
		Location: "48.1189,11.4289", // lat,lon
		Created:  time.Date(2012, 10, 10, 8, 12, 03, 0, time.UTC),
	}
	tweet3 := tweet{
		User:     "sandrae",
		Retweets: 12,
		Message:  "Cycling is fun.",
		Tags:     []string{"sports", "cycling"},
		Location: "47.7167,11.7167", // lat,lon
		Created:  time.Date(2011, 11, 11, 10, 58, 12, 0, time.UTC),
	}

	// Add all documents
	_, err := client.Index().Index(testIndexName).Id("1").BodyJson(&tweet1).Do(context.TODO())
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.Index().Index(testIndexName).Id("2").BodyJson(&tweet2).Do(context.TODO())
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.Index().Index(testIndexName).Id("3").BodyJson(&tweet3).Do(context.TODO())
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.Refresh().Index(testIndexName).Do(context.TODO())
	if err != nil {
		t.Fatal(err)
	}

	count, err := client.Count(testIndexName).Do(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if want, have := int64(3), count; want != have {
		t.Fatalf("expected %d documents, got %d", want, have)
	}

	// Match all should return all documents
	all := NewMatchAllQuery()

	// Terms Aggregate by user name
	globalAgg := NewGlobalAggregation()
	usersAgg := NewTermsAggregation().Field("user").Size(10).OrderByCountDesc()
	retweetsAgg := NewTermsAggregation().Field("retweets").Size(10)
	avgRetweetsAgg := NewAvgAggregation().Field("retweets")
	avgRetweetsWithMetaAgg := NewAvgAggregation().Field("retweetsMeta").Meta(map[string]interface{}{"meta": true})
	weightedAvgRetweetsAgg := NewWeightedAvgAggregation().
		Value(&MultiValuesSourceFieldConfig{FieldName: "retweets"}).
		Weight(&MultiValuesSourceFieldConfig{FieldName: "weight", Missing: 1.0})
	minRetweetsAgg := NewMinAggregation().Field("retweets")
	maxRetweetsAgg := NewMaxAggregation().Field("retweets")
	sumRetweetsAgg := NewSumAggregation().Field("retweets")
	statsRetweetsAgg := NewStatsAggregation().Field("retweets")
	extstatsRetweetsAgg := NewExtendedStatsAggregation().Field("retweets")
	valueCountRetweetsAgg := NewValueCountAggregation().Field("retweets")
	percentilesRetweetsAgg := NewPercentilesAggregation().Field("retweets")
	percentileRanksRetweetsAgg := NewPercentileRanksAggregation().Field("retweets").Values(25, 50, 75)
	cardinalityAgg := NewCardinalityAggregation().Field("user")
	significantTermsAgg := NewSignificantTermsAggregation().Field("message")
	samplerAgg := NewSamplerAggregation().SubAggregation("tagged_with", NewTermsAggregation().Field("tags"))
	diversifiedSamplerAgg := NewDiversifiedSamplerAggregation().Field("user").SubAggregation("tagged_with", NewSignificantTermsAggregation().Field("tags"))
	retweetsRangeAgg := NewRangeAggregation().Field("retweets").Lt(10).Between(10, 100).Gt(100)
	retweetsKeyedRangeAgg := NewRangeAggregation().Field("retweets").Keyed(true).Lt(10).Between(10, 100).Gt(100)
	dateRangeAgg := NewDateRangeAggregation().Field("created").Lt("2012-01-01").Between("2012-01-01", "2013-01-01").Gt("2013-01-01")
	missingTagsAgg := NewMissingAggregation().Field("tags")
	retweetsHistoAgg := NewHistogramAggregation().Field("retweets").Interval(100)
	autoDateHistoAgg := NewAutoDateHistogramAggregation().Field("created").Buckets(2).Missing("1900-01-01").Format("yyyy-MM-dd")
	dateHistoAgg := NewDateHistogramAggregation().Field("created").Interval("year")
	dateHistoKeyedAgg := NewDateHistogramAggregation().Field("created").Interval("year").Keyed(true)
	retweetsFilterAgg := NewFilterAggregation().Filter(
		NewRangeQuery("created").Gte("2012-01-01").Lte("2012-12-31")).
		SubAggregation("avgRetweetsSub", NewAvgAggregation().Field("retweets"))
	queryFilterAgg := NewFilterAggregation().Filter(NewTermQuery("tags", "golang"))
	topTagsHitsAgg := NewTopHitsAggregation().Sort("created", false).Size(5).FetchSource(true)
	topTagsAgg := NewTermsAggregation().Field("tags").Size(3).SubAggregation("top_tag_hits", topTagsHitsAgg)
	geoBoundsAgg := NewGeoBoundsAggregation().Field("location")
	geoHashAgg := NewGeoHashGridAggregation().Field("location").Precision(5)
	geoCentroidAgg := NewGeoCentroidAggregation().Field("location")

	// Run query
	builder := client.Search().Index(testIndexName).Query(all).Pretty(true)
	builder = builder.Aggregation("global", globalAgg)
	builder = builder.Aggregation("users", usersAgg)
	builder = builder.Aggregation("retweets", retweetsAgg)
	builder = builder.Aggregation("avgRetweets", avgRetweetsAgg)
	builder = builder.Aggregation("avgRetweetsWithMeta", avgRetweetsWithMetaAgg)
	builder = builder.Aggregation("weightedAvgRetweets", weightedAvgRetweetsAgg)
	builder = builder.Aggregation("minRetweets", minRetweetsAgg)
	builder = builder.Aggregation("maxRetweets", maxRetweetsAgg)
	builder = builder.Aggregation("sumRetweets", sumRetweetsAgg)
	builder = builder.Aggregation("statsRetweets", statsRetweetsAgg)
	builder = builder.Aggregation("extstatsRetweets", extstatsRetweetsAgg)
	builder = builder.Aggregation("valueCountRetweets", valueCountRetweetsAgg)
	builder = builder.Aggregation("percentilesRetweets", percentilesRetweetsAgg)
	builder = builder.Aggregation("percentileRanksRetweets", percentileRanksRetweetsAgg)
	builder = builder.Aggregation("usersCardinality", cardinalityAgg)
	builder = builder.Aggregation("significantTerms", significantTermsAgg)
	builder = builder.Aggregation("sample", samplerAgg)
	builder = builder.Aggregation("diversified_sampler", diversifiedSamplerAgg)
	builder = builder.Aggregation("retweetsRange", retweetsRangeAgg)
	builder = builder.Aggregation("retweetsKeyedRange", retweetsKeyedRangeAgg)
	builder = builder.Aggregation("dateRange", dateRangeAgg)
	builder = builder.Aggregation("missingTags", missingTagsAgg)
	builder = builder.Aggregation("retweetsHisto", retweetsHistoAgg)
	builder = builder.Aggregation("autoDateHisto", autoDateHistoAgg)
	builder = builder.Aggregation("dateHisto", dateHistoAgg)
	builder = builder.Aggregation("dateHistoKeyed", dateHistoKeyedAgg)
	builder = builder.Aggregation("retweetsFilter", retweetsFilterAgg)
	builder = builder.Aggregation("queryFilter", queryFilterAgg)
	builder = builder.Aggregation("top-tags", topTagsAgg)
	builder = builder.Aggregation("viewport", geoBoundsAgg)
	builder = builder.Aggregation("geohashed", geoHashAgg)
	builder = builder.Aggregation("centroid", geoCentroidAgg)
	// Unnamed filters
	countByUserAgg := NewFiltersAggregation().
		Filters(NewTermQuery("user", "olivere"), NewTermQuery("user", "sandrae"))
	builder = builder.Aggregation("countByUser", countByUserAgg)
	// Named filters
	countByUserAgg2 := NewFiltersAggregation().
		FilterWithName("olivere", NewTermQuery("user", "olivere")).
		FilterWithName("sandrae", NewTermQuery("user", "sandrae"))
	builder = builder.Aggregation("countByUser2", countByUserAgg2)
	// AdjacencyMatrix
	adjacencyMatrixAgg := NewAdjacencyMatrixAggregation().
		Filters("groupA", NewTermQuery("user", "olivere")).
		Filters("groupB", NewTermQuery("user", "sandrae"))
	builder = builder.Aggregation("interactions", adjacencyMatrixAgg)
	// AvgBucket
	dateHisto := NewDateHistogramAggregation().Field("created").Interval("year")
	dateHisto = dateHisto.SubAggregation("sumOfRetweets", NewSumAggregation().Field("retweets"))
	builder = builder.Aggregation("avgBucketDateHisto", dateHisto)
	builder = builder.Aggregation("avgSumOfRetweets", NewAvgBucketAggregation().BucketsPath("avgBucketDateHisto>sumOfRetweets"))
	// MinBucket
	dateHisto = NewDateHistogramAggregation().Field("created").Interval("year")
	dateHisto = dateHisto.SubAggregation("sumOfRetweets", NewSumAggregation().Field("retweets"))
	builder = builder.Aggregation("minBucketDateHisto", dateHisto)
	builder = builder.Aggregation("minBucketSumOfRetweets", NewMinBucketAggregation().BucketsPath("minBucketDateHisto>sumOfRetweets"))
	// MaxBucket
	dateHisto = NewDateHistogramAggregation().Field("created").Interval("year")
	dateHisto = dateHisto.SubAggregation("sumOfRetweets", NewSumAggregation().Field("retweets"))
	builder = builder.Aggregation("maxBucketDateHisto", dateHisto)
	builder = builder.Aggregation("maxBucketSumOfRetweets", NewMaxBucketAggregation().BucketsPath("maxBucketDateHisto>sumOfRetweets"))
	// SumBucket
	dateHisto = NewDateHistogramAggregation().Field("created").Interval("year")
	dateHisto = dateHisto.SubAggregation("sumOfRetweets", NewSumAggregation().Field("retweets"))
	builder = builder.Aggregation("sumBucketDateHisto", dateHisto)
	builder = builder.Aggregation("sumBucketSumOfRetweets", NewSumBucketAggregation().BucketsPath("sumBucketDateHisto>sumOfRetweets"))
	// MovAvg
	dateHisto = NewDateHistogramAggregation().Field("created").Interval("year")
	dateHisto = dateHisto.SubAggregation("sumOfRetweets", NewSumAggregation().Field("retweets"))
	dateHisto = dateHisto.SubAggregation("movingAvg", NewMovAvgAggregation().BucketsPath("sumOfRetweets"))
	dateHisto = dateHisto.SubAggregation("movingFn", NewMovFnAggregation("sumOfRetweets", NewScript("MovingFunctions.sum(values)"), 10))
	builder = builder.Aggregation("movingAvgDateHisto", dateHisto)
	searchResult, err := builder.Pretty(true).Do(context.TODO())
	if err != nil {
		t.Fatal(err)
	}
	if searchResult.Hits == nil {
		t.Errorf("expected Hits != nil; got: nil")
	}
	if searchResult.TotalHits() != 3 {
		t.Errorf("expected TotalHits() = %d; got: %d", 3, searchResult.TotalHits())
	}
	if len(searchResult.Hits.Hits) != 3 {
		t.Errorf("expected len(Hits.Hits) = %d; got: %d", 3, len(searchResult.Hits.Hits))
	}
	agg := searchResult.Aggregations
	if agg == nil {
		t.Fatalf("expected Aggregations != nil; got: nil")
	}

	// Search for non-existent aggregate should return (nil, false)
	unknownAgg, found := agg.Terms("no-such-aggregate")
	if found {
		t.Errorf("expected unknown aggregation to not be found; got: %v", found)
	}
	if unknownAgg != nil {
		t.Errorf("expected unknown aggregation to return %v; got %v", nil, unknownAgg)
	}

	// Global
	globalAggRes, found := agg.Global("global")
	if !found {
		t.Errorf("expected %v; got: %v", true, found)
	}
	if globalAggRes == nil {
		t.Fatalf("expected != nil; got: nil")
	}
	if globalAggRes.DocCount != 3 {
		t.Errorf("expected DocCount = %d; got: %d", 3, globalAggRes.DocCount)
	}

	// Search for existent aggregate (by name) should return (aggregate, true)
	termsAggRes, found := agg.Terms("users")
	if !found {
		t.Errorf("expected %v; got: %v", true, found)
	}
	if termsAggRes == nil {
		t.Fatalf("expected != nil; got: nil")
	}
	if len(termsAggRes.Buckets) != 2 {
		t.Fatalf("expected %d; got: %d", 2, len(termsAggRes.Buckets))
	}
	if termsAggRes.Buckets[0].Key != "olivere" {
		t.Errorf("expected %q; got: %q", "olivere", termsAggRes.Buckets[0].Key)
	}
	if termsAggRes.Buckets[0].DocCount != 2 {
		t.Errorf("expected %d; got: %d", 2, termsAggRes.Buckets[0].DocCount)
	}
	if termsAggRes.Buckets[1].Key != "sandrae" {
		t.Errorf("expected %q; got: %q", "sandrae", termsAggRes.Buckets[1].Key)
	}
	if termsAggRes.Buckets[1].DocCount != 1 {
		t.Errorf("expected %d; got: %d", 1, termsAggRes.Buckets[1].DocCount)
	}

	// A terms aggregate with keys that are not strings
	retweetsAggRes, found := agg.Terms("retweets")
	if !found {
		t.Errorf("expected %v; got: %v", true, found)
	}
	if retweetsAggRes == nil {
		t.Fatalf("expected != nil; got: nil")
	}
	if len(retweetsAggRes.Buckets) != 3 {
		t.Fatalf("expected %d; got: %d", 3, len(retweetsAggRes.Buckets))
	}

	if retweetsAggRes.Buckets[0].Key != float64(0) {
		t.Errorf("expected %v; got: %v", float64(0), retweetsAggRes.Buckets[0].Key)
	}
	if got, err := retweetsAggRes.Buckets[0].KeyNumber.Int64(); err != nil {
		t.Errorf("expected %d; got: %v", 0, retweetsAggRes.Buckets[0].Key)
	} else if got != 0 {
		t.Errorf("expected %d; got: %d", 0, got)
	}
	if retweetsAggRes.Buckets[0].KeyNumber != "0" {
		t.Errorf("expected %q; got: %q", "0", retweetsAggRes.Buckets[0].KeyNumber)
	}
	if retweetsAggRes.Buckets[0].DocCount != 1 {
		t.Errorf("expected %d; got: %d", 1, retweetsAggRes.Buckets[0].DocCount)
	}

	if retweetsAggRes.Buckets[1].Key != float64(12) {
		t.Errorf("expected %v; got: %v", float64(12), retweetsAggRes.Buckets[1].Key)
	}
	if got, err := retweetsAggRes.Buckets[1].KeyNumber.Int64(); err != nil {
		t.Errorf("expected %d; got: %v", 0, retweetsAggRes.Buckets[1].KeyNumber)
	} else if got != 12 {
		t.Errorf("expected %d; got: %d", 12, got)
	}
	if retweetsAggRes.Buckets[1].KeyNumber != "12" {
		t.Errorf("expected %q; got: %q", "12", retweetsAggRes.Buckets[1].KeyNumber)
	}
	if retweetsAggRes.Buckets[1].DocCount != 1 {
		t.Errorf("expected %d; got: %d", 1, retweetsAggRes.Buckets[1].DocCount)
	}

	if retweetsAggRes.Buckets[2].Key != float64(108) {
		t.Errorf("expected %v; got: %v", float64(108), retweetsAggRes.Buckets[2].Key)
	}
	if got, err := retweetsAggRes.Buckets[2].KeyNumber.Int64(); err != nil {
		t.Errorf("expected %d; got: %v", 108, retweetsAggRes.Buckets[2].KeyNumber)
	} else if got != 108 {
		t.Errorf("expected %d; got: %d", 108, got)
	}
	if retweetsAggRes.Buckets[2].KeyNumber != "108" {
		t.Errorf("expected %q; got: %q", "108", retweetsAggRes.Buckets[2].KeyNumber)
	}
	if retweetsAggRes.Buckets[2].DocCount != 1 {
		t.Errorf("expected %d; got: %d", 1, retweetsAggRes.Buckets[2].DocCount)
	}

	// avgRetweets
	avgAggRes, found := agg.Avg("avgRetweets")
	if !found {
		t.Errorf("expected %v; got: %v", true, found)
	}
	if avgAggRes == nil {
		t.Fatalf("expected != nil; got: nil")
	}
	if avgAggRes.Value == nil {
		t.Fatalf("expected != nil; got: %v", *avgAggRes.Value)
	}
	if *avgAggRes.Value != 40.0 {
		t.Errorf("expected %v; got: %v", 40.0, *avgAggRes.Value)
	}

	// avgRetweetsWithMeta
	avgMetaAggRes, found := agg.Avg("avgRetweetsWithMeta")
	if !found {
		t.Errorf("expected %v; got: %v", true, found)
	}
	if avgMetaAggRes == nil {
		t.Fatalf("expected != nil; got: nil")
	}
	if avgMetaAggRes.Meta == nil {
		t.Fatalf("expected != nil; got: %v", avgMetaAggRes.Meta)
	}
	metaDataValue, found := avgMetaAggRes.Meta["meta"]
	if !found {
		t.Fatalf("expected to return meta data key %q; got: %v", "meta", found)
	}
	if flag, ok := metaDataValue.(bool); !ok {
		t.Fatalf("expected to return meta data key type %T; got: %T", true, metaDataValue)
	} else if flag != true {
		t.Fatalf("expected to return meta data key value %v; got: %v", true, flag)
	}

	// weightedAvgRetweets
	weightedAvgRes, found := agg.Avg("weightedAvgRetweets")
	if !found {
		t.Errorf("expected %v; got: %v", true, found)
	}
	if weightedAvgRes == nil {
		t.Fatalf("expected != nil; got: nil")
	}
	if weightedAvgRes.Value == nil {
		t.Fatalf("expected != nil; got: %v", *weightedAvgRes.Value)
	}
	if *weightedAvgRes.Value != 40.0 {
		t.Errorf("expected %v; got: %v", 40.0, *weightedAvgRes.Value)
	}

	// minRetweets
	minAggRes, found := agg.Min("minRetweets")
	if !found {
		t.Errorf("expected %v; got: %v", true, found)
	}
	if minAggRes == nil {
		t.Fatalf("expected != nil; got: nil")
	}
	if minAggRes.Value == nil {
		t.Fatalf("expected != nil; got: %v", *minAggRes.Value)
	}
	if *minAggRes.Value != 0.0 {
		t.Errorf("expected %v; got: %v", 0.0, *minAggRes.Value)
	}

	// maxRetweets
	maxAggRes, found := agg.Max("maxRetweets")
	if !found {
		t.Errorf("expected %v; got: %v", true, found)
	}
	if maxAggRes == nil {
		t.Fatalf("expected != nil; got: nil")
	}
	if maxAggRes.Value == nil {
		t.Fatalf("expected != nil; got: %v", *maxAggRes.Value)
	}
	if *maxAggRes.Value != 108.0 {
		t.Errorf("expected %v; got: %v", 108.0, *maxAggRes.Value)
	}

	// sumRetweets
	sumAggRes, found := agg.Sum("sumRetweets")
	if !found {
		t.Errorf("expected %v; got: %v", true, found)
	}
	if sumAggRes == nil {
		t.Fatalf("expected != nil; got: nil")
	}
	if sumAggRes.Value == nil {
		t.Fatalf("expected != nil; got: %v", *sumAggRes.Value)
	}
	if *sumAggRes.Value != 120.0 {
		t.Errorf("expected %v; got: %v", 120.0, *sumAggRes.Value)
	}

	// statsRetweets
	statsAggRes, found := agg.Stats("statsRetweets")
	if !found {
		t.Errorf("expected %v; got: %v", true, found)
	}
	if statsAggRes == nil {
		t.Fatalf("expected != nil; got: nil")
	}
	if statsAggRes.Count != 3 {
		t.Errorf("expected %d; got: %d", 3, statsAggRes.Count)
	}
	if statsAggRes.Min == nil {
		t.Fatalf("expected != nil; got: %v", *statsAggRes.Min)
	}
	if *statsAggRes.Min != 0.0 {
		t.Errorf("expected %v; got: %v", 0.0, *statsAggRes.Min)
	}
	if statsAggRes.Max == nil {
		t.Fatalf("expected != nil; got: %v", *statsAggRes.Max)
	}
	if *statsAggRes.Max != 108.0 {
		t.Errorf("expected %v; got: %v", 108.0, *statsAggRes.Max)
	}
	if statsAggRes.Avg == nil {
		t.Fatalf("expected != nil; got: %v", *statsAggRes.Avg)
	}
	if *statsAggRes.Avg != 40.0 {
		t.Errorf("expected %v; got: %v", 40.0, *statsAggRes.Avg)
	}
	if statsAggRes.Sum == nil {
		t.Fatalf("expected != nil; got: %v", *statsAggRes.Sum)
	}
	if *statsAggRes.Sum != 120.0 {
		t.Errorf("expected %v; got: %v", 120.0, *statsAggRes.Sum)
	}

	// extstatsRetweets
	extStatsAggRes, found := agg.ExtendedStats("extstatsRetweets")
	if !found {
		t.Errorf("expected %v; got: %v", true, found)
	}
	if extStatsAggRes == nil {
		t.Fatalf("expected != nil; got: nil")
	}
	if extStatsAggRes.Count != 3 {
		t.Errorf("expected %d; got: %d", 3, extStatsAggRes.Count)
	}
	if extStatsAggRes.Min == nil {
		t.Fatalf("expected != nil; got: %v", *extStatsAggRes.Min)
	}
	if *extStatsAggRes.Min != 0.0 {
		t.Errorf("expected %v; got: %v", 0.0, *extStatsAggRes.Min)
	}
	if extStatsAggRes.Max == nil {
		t.Fatalf("expected != nil; got: %v", *extStatsAggRes.Max)
	}
	if *extStatsAggRes.Max != 108.0 {
		t.Errorf("expected %v; got: %v", 108.0, *extStatsAggRes.Max)
	}
	if extStatsAggRes.Avg == nil {
		t.Fatalf("expected != nil; got: %v", *extStatsAggRes.Avg)
	}
	if *extStatsAggRes.Avg != 40.0 {
		t.Errorf("expected %v; got: %v", 40.0, *extStatsAggRes.Avg)
	}
	if extStatsAggRes.Sum == nil {
		t.Fatalf("expected != nil; got: %v", *extStatsAggRes.Sum)
	}
	if *extStatsAggRes.Sum != 120.0 {
		t.Errorf("expected %v; got: %v", 120.0, *extStatsAggRes.Sum)
	}
	if extStatsAggRes.SumOfSquares == nil {
		t.Fatalf("expected != nil; got: %v", *extStatsAggRes.SumOfSquares)
	}
	if *extStatsAggRes.SumOfSquares != 11808.0 {
		t.Errorf("expected %v; got: %v", 11808.0, *extStatsAggRes.SumOfSquares)
	}
	if extStatsAggRes.Variance == nil {
		t.Fatalf("expected != nil; got: %v", *extStatsAggRes.Variance)
	}
	if *extStatsAggRes.Variance != 2336.0 {
		t.Errorf("expected %v; got: %v", 2336.0, *extStatsAggRes.Variance)
	}
	if extStatsAggRes.StdDeviation == nil {
		t.Fatalf("expected != nil; got: %v", *extStatsAggRes.StdDeviation)
	}
	if *extStatsAggRes.StdDeviation != 48.33218389437829 {
		t.Errorf("expected %v; got: %v", 48.33218389437829, *extStatsAggRes.StdDeviation)
	}

	// valueCountRetweets
	valueCountAggRes, found := agg.ValueCount("valueCountRetweets")
	if !found {
		t.Errorf("expected %v; got: %v", true, found)
	}
	if valueCountAggRes == nil {
		t.Fatalf("expected != nil; got: nil")
	}
	if valueCountAggRes.Value == nil {
		t.Fatalf("expected != nil; got: %v", *valueCountAggRes.Value)
	}
	if *valueCountAggRes.Value != 3.0 {
		t.Errorf("expected %v; got: %v", 3.0, *valueCountAggRes.Value)
	}

	// percentilesRetweets
	percentilesAggRes, found := agg.Percentiles("percentilesRetweets")
	if !found {
		t.Errorf("expected %v; got: %v", true, found)
	}
	if percentilesAggRes == nil {
		t.Fatalf("expected != nil; got: nil")
	}
	// ES 1.4.x returns  7: {"1.0":...}
	// ES 1.5.0 returns 14: {"1.0":..., "1.0_as_string":...}
	// So we're relaxing the test here.
	if len(percentilesAggRes.Values) == 0 {
		t.Errorf("expected at least %d value; got: %d\nValues are: %#v", 1, len(percentilesAggRes.Values), percentilesAggRes.Values)
	}
	if _, found := percentilesAggRes.Values["0.0"]; found {
		t.Errorf("expected %v; got: %v", false, found)
	}
	if percentilesAggRes.Values["1.0"] != 0.0 {
		t.Errorf("expected %v; got: %v", 0.0, percentilesAggRes.Values["1.0"])
	}
	if percentilesAggRes.Values["5.0"] != 0.0 {
		t.Errorf("expected %v; got: %v", 0.0, percentilesAggRes.Values["1.0"])
	}
	if percentilesAggRes.Values["25.0"] != 3.0 {
		t.Errorf("expected %v; got: %v", 3.0, percentilesAggRes.Values["25.0"])
	}
	if percentilesAggRes.Values["50.0"] != 12.0 {
		t.Errorf("expected %v; got: %v", 12.0, percentilesAggRes.Values["50.0"])
	}
	if percentilesAggRes.Values["75.0"] != 84.0 {
		t.Errorf("expected %v; got: %v", 84.0, percentilesAggRes.Values["75.0"])
	}
	if percentilesAggRes.Values["95.0"] != 108.0 {
		t.Errorf("expected %v; got: %v", 108.0, percentilesAggRes.Values["95.0"])
	}
	if percentilesAggRes.Values["99.0"] != 108.0 {
		t.Errorf("expected %v; got: %v", 108.0, percentilesAggRes.Values["99.0"])
	}

	// percentileRanksRetweets
	percentileRanksAggRes, found := agg.PercentileRanks("percentileRanksRetweets")
	if !found {
		t.Errorf("expected %v; got: %v", true, found)
	}
	if percentileRanksAggRes == nil {
		t.Fatalf("expected != nil; got: nil")
	}
	if len(percentileRanksAggRes.Values) == 0 {
		t.Errorf("expected at least %d value; got %d\nValues are: %#v", 1, len(percentileRanksAggRes.Values), percentileRanksAggRes.Values)
	}
	if _, found := percentileRanksAggRes.Values["0.0"]; found {
		t.Errorf("expected %v; got: %v", true, found)
	}
	if percentileRanksAggRes.Values["25.0"] != 45.06172839506173 {
		t.Errorf("expected %v; got: %v", 45.06172839506173, percentileRanksAggRes.Values["25.0"])
	}
	if percentileRanksAggRes.Values["50.0"] != 60.49382716049383 {
		t.Errorf("expected %v; got: %v", 60.49382716049383, percentileRanksAggRes.Values["50.0"])
	}
	if percentileRanksAggRes.Values["75.0"] != 100.0 {
		t.Errorf("expected %v; got: %v", 100.0, percentileRanksAggRes.Values["75.0"])
	}

	// usersCardinality
	cardAggRes, found := agg.Cardinality("usersCardinality")
	if !found {
		t.Errorf("expected %v; got: %v", true, found)
	}
	if cardAggRes == nil {
		t.Fatalf("expected != nil; got: nil")
	}
	if cardAggRes.Value == nil {
		t.Fatalf("expected != nil; got: %v", *cardAggRes.Value)
	}
	if *cardAggRes.Value != 2 {
		t.Errorf("expected %v; got: %v", 2, *cardAggRes.Value)
	}

	// retweetsFilter
	filterAggRes, found := agg.Filter("retweetsFilter")
	if !found {
		t.Errorf("expected %v; got: %v", true, found)
	}
	if filterAggRes == nil {
		t.Fatalf("expected != nil; got: nil")
	}
	if filterAggRes.DocCount != 2 {
		t.Fatalf("expected %v; got: %v", 2, filterAggRes.DocCount)
	}

	// Retrieve sub-aggregation
	avgRetweetsAggRes, found := filterAggRes.Avg("avgRetweetsSub")
	if !found {
		t.Error("expected sub-aggregation \"avgRetweets\" to be found; got false")
	}
	if avgRetweetsAggRes == nil {
		t.Fatal("expected sub-aggregation \"avgRetweets\"; got nil")
	}
	if avgRetweetsAggRes.Value == nil {
		t.Fatalf("expected != nil; got: %v", avgRetweetsAggRes.Value)
	}
	if *avgRetweetsAggRes.Value != 54.0 {
		t.Errorf("expected %v; got: %v", 54.0, *avgRetweetsAggRes.Value)
	}

	// queryFilter
	queryFilterAggRes, found := agg.Filter("queryFilter")
	if !found {
		t.Errorf("expected %v; got: %v", true, found)
	}
	if queryFilterAggRes == nil {
		t.Fatalf("expected != nil; got: nil")
	}
	if queryFilterAggRes.DocCount != 2 {
		t.Fatalf("expected %v; got: %v", 2, queryFilterAggRes.DocCount)
	}

	// significantTerms
	stAggRes, found := agg.SignificantTerms("significantTerms")
	if !found {
		t.Errorf("expected %v; got: %v", true, found)
	}
	if stAggRes == nil {
		t.Fatalf("expected != nil; got: nil")
	}
	if stAggRes.DocCount != 3 {
		t.Errorf("expected %v; got: %v", 3, stAggRes.DocCount)
	}
	if len(stAggRes.Buckets) != 0 {
		t.Errorf("expected %v; got: %v", 0, len(stAggRes.Buckets))
	}

	// sampler
	samplerAggRes, found := agg.Sampler("sample")
	if !found {
		t.Errorf("expected %v; got: %v", true, found)
	}
	if samplerAggRes == nil {
		t.Fatalf("expected != nil; got: nil")
	}
	if samplerAggRes.DocCount != 3 {
		t.Errorf("expected %v; got: %v", 3, samplerAggRes.DocCount)
	}
	sub, found := samplerAggRes.Aggregations["tagged_with"]
	if !found {
		t.Fatalf("expected sub aggregation %q", "tagged_with")
	}
	if sub == nil {
		t.Fatalf("expected sub aggregation %q; got: %v", "tagged_with", sub)
	}

	// diversified_sampler
	diversifiedSamplerAggRes, found := agg.DiversifiedSampler("diversified_sampler")
	if !found {
		t.Errorf("expected %v; got: %v", true, found)
	}
	if diversifiedSamplerAggRes == nil {
		t.Fatalf("expected != nil; got: nil")
	}
	if diversifiedSamplerAggRes.DocCount != 2 {
		t.Errorf("expected %v; got: %v", 2, diversifiedSamplerAggRes.DocCount)
	}
	subAgg, found := samplerAggRes.Aggregations["tagged_with"]
	if !found {
		t.Fatalf("expected sub aggregation %q", "tagged_with")
	}
	if subAgg == nil {
		t.Fatalf("expected sub aggregation %q; got: %v", "tagged_with", subAgg)
	}

	// retweetsRange
	rangeAggRes, found := agg.Range("retweetsRange")
	if !found {
		t.Errorf("expected %v; got: %v", true, found)
	}
	if rangeAggRes == nil {
		t.Fatal("expected != nil; got: nil")
	}
	if len(rangeAggRes.Buckets) != 3 {
		t.Fatalf("expected %d; got: %d", 3, len(rangeAggRes.Buckets))
	}
	if rangeAggRes.Buckets[0].DocCount != 1 {
		t.Errorf("expected %d; got: %d", 1, rangeAggRes.Buckets[0].DocCount)
	}
	if rangeAggRes.Buckets[1].DocCount != 1 {
		t.Errorf("expected %d; got: %d", 1, rangeAggRes.Buckets[1].DocCount)
	}
	if rangeAggRes.Buckets[2].DocCount != 1 {
		t.Errorf("expected %d; got: %d", 1, rangeAggRes.Buckets[2].DocCount)
	}

	// retweetsKeyedRange
	keyedRangeAggRes, found := agg.KeyedRange("retweetsKeyedRange")
	if !found {
		t.Errorf("expected %v; got: %v", true, found)
	}
	if keyedRangeAggRes == nil {
		t.Fatal("expected != nil; got: nil")
	}
	if len(keyedRangeAggRes.Buckets) != 3 {
		t.Fatalf("expected %d; got: %d", 3, len(keyedRangeAggRes.Buckets))
	}
	_, found = keyedRangeAggRes.Buckets["no-such-key"]
	if found {
		t.Fatalf("expected bucket to not be found; got: %v", found)
	}
	bucket, found := keyedRangeAggRes.Buckets["*-10.0"]
	if !found {
		t.Fatalf("expected bucket to be found; got: %v", found)
	}
	if bucket.DocCount != 1 {
		t.Errorf("expected %d; got: %d", 1, bucket.DocCount)
	}
	bucket, found = keyedRangeAggRes.Buckets["10.0-100.0"]
	if !found {
		t.Fatalf("expected bucket to be found; got: %v", found)
	}
	if bucket.DocCount != 1 {
		t.Errorf("expected %d; got: %d", 1, bucket.DocCount)
	}
	bucket, found = keyedRangeAggRes.Buckets["100.0-*"]
	if !found {
		t.Fatalf("expected bucket to be found; got: %v", found)
	}
	if bucket.DocCount != 1 {
		t.Errorf("expected %d; got: %d", 1, bucket.DocCount)
	}

	// dateRange
	dateRangeRes, found := agg.DateRange("dateRange")
	if !found {
		t.Errorf("expected %v; got: %v", true, found)
	}
	if dateRangeRes == nil {
		t.Fatal("expected != nil; got: nil")
	}
	if dateRangeRes.Buckets[0].DocCount != 1 {
		t.Errorf("expected %d; got: %d", 1, dateRangeRes.Buckets[0].DocCount)
	}
	if dateRangeRes.Buckets[0].From != nil {
		t.Fatal("expected From to be nil")
	}
	if dateRangeRes.Buckets[0].To == nil {
		t.Fatal("expected To to be != nil")
	}
	if *dateRangeRes.Buckets[0].To != 1.325376e+12 {
		t.Errorf("expected %v; got: %v", 1.325376e+12, *dateRangeRes.Buckets[0].To)
	}
	if dateRangeRes.Buckets[0].ToAsString != "2012-01-01T00:00:00.000Z" {
		t.Errorf("expected %q; got: %q", "2012-01-01T00:00:00.000Z", dateRangeRes.Buckets[0].ToAsString)
	}
	if dateRangeRes.Buckets[1].DocCount != 2 {
		t.Errorf("expected %d; got: %d", 2, dateRangeRes.Buckets[1].DocCount)
	}
	if dateRangeRes.Buckets[1].From == nil {
		t.Fatal("expected From to be != nil")
	}
	if *dateRangeRes.Buckets[1].From != 1.325376e+12 {
		t.Errorf("expected From = %v; got: %v", 1.325376e+12, *dateRangeRes.Buckets[1].From)
	}
	if dateRangeRes.Buckets[1].FromAsString != "2012-01-01T00:00:00.000Z" {
		t.Errorf("expected FromAsString = %q; got: %q", "2012-01-01T00:00:00.000Z", dateRangeRes.Buckets[1].FromAsString)
	}
	if dateRangeRes.Buckets[1].To == nil {
		t.Fatal("expected To to be != nil")
	}
	if *dateRangeRes.Buckets[1].To != 1.3569984e+12 {
		t.Errorf("expected To = %v; got: %v", 1.3569984e+12, *dateRangeRes.Buckets[1].To)
	}
	if dateRangeRes.Buckets[1].ToAsString != "2013-01-01T00:00:00.000Z" {
		t.Errorf("expected ToAsString = %q; got: %q", "2013-01-01T00:00:00.000Z", dateRangeRes.Buckets[1].ToAsString)
	}
	if dateRangeRes.Buckets[2].DocCount != 0 {
		t.Errorf("expected %d; got: %d", 0, dateRangeRes.Buckets[2].DocCount)
	}
	if dateRangeRes.Buckets[2].To != nil {
		t.Fatal("expected To to be nil")
	}
	if dateRangeRes.Buckets[2].From == nil {
		t.Fatal("expected From to be != nil")
	}
	if *dateRangeRes.Buckets[2].From != 1.3569984e+12 {
		t.Errorf("expected %v; got: %v", 1.3569984e+12, *dateRangeRes.Buckets[2].From)
	}
	if dateRangeRes.Buckets[2].FromAsString != "2013-01-01T00:00:00.000Z" {
		t.Errorf("expected %q; got: %q", "2013-01-01T00:00:00.000Z", dateRangeRes.Buckets[2].FromAsString)
	}

	// missingTags
	missingRes, found := agg.Missing("missingTags")
	if !found {
		t.Errorf("expected %v; got: %v", true, found)
	}
	if missingRes == nil {
		t.Fatalf("expected != nil; got: nil")
	}
	if missingRes.DocCount != 0 {
		t.Errorf("expected searchResult.Aggregations[\"missingTags\"].DocCount = %v; got %v", 0, missingRes.DocCount)
	}

	// retweetsHisto
	histoRes, found := agg.Histogram("retweetsHisto")
	if !found {
		t.Errorf("expected %v; got: %v", true, found)
	}
	if histoRes == nil {
		t.Fatalf("expected != nil; got: nil")
	}
	if len(histoRes.Buckets) != 2 {
		t.Fatalf("expected %d; got: %d", 2, len(histoRes.Buckets))
	}
	if histoRes.Buckets[0].DocCount != 2 {
		t.Errorf("expected %d; got: %d", 2, histoRes.Buckets[0].DocCount)
	}
	if histoRes.Buckets[0].Key != 0.0 {
		t.Errorf("expected %v; got: %v", 0.0, histoRes.Buckets[0].Key)
	}
	if histoRes.Buckets[1].DocCount != 1 {
		t.Errorf("expected %d; got: %d", 1, histoRes.Buckets[1].DocCount)
	}
	if histoRes.Buckets[1].Key != 100.0 {
		t.Errorf("expected %v; got: %+v", 100.0, histoRes.Buckets[1].Key)
	}

	// autoDateHisto
	autoDateHistoRes, found := agg.DateHistogram("autoDateHisto")
	if !found {
		t.Errorf("expected %v; got: %v", true, found)
	}
	if autoDateHistoRes == nil {
		t.Fatal("expected != nil; got: nil")
	}
	if len(autoDateHistoRes.Buckets) != 2 {
		t.Fatalf("expected %d; got: %d", 2, len(autoDateHistoRes.Buckets))
	}
	if autoDateHistoRes.Interval == nil {
		t.Fatalf("expected an interval; got: %v", autoDateHistoRes.Interval)
	}

	// dateHisto
	dateHistoRes, found := agg.DateHistogram("dateHisto")
	if !found {
		t.Errorf("expected %v; got: %v", true, found)
	}
	if dateHistoRes == nil {
		t.Fatal("expected != nil; got: nil")
	}
	if len(dateHistoRes.Buckets) != 2 {
		t.Fatalf("expected %d; got: %d", 2, len(dateHistoRes.Buckets))
	}
	if dateHistoRes.Buckets[0].DocCount != 1 {
		t.Errorf("expected %d; got: %d", 1, dateHistoRes.Buckets[0].DocCount)
	}
	if dateHistoRes.Buckets[0].Key != 1.29384e+12 {
		t.Errorf("expected %v; got: %v", 1.29384e+12, dateHistoRes.Buckets[0].Key)
	}
	if dateHistoRes.Buckets[0].KeyAsString == nil {
		t.Fatalf("expected != nil; got: %v", dateHistoRes.Buckets[0].KeyAsString)
	}
	if *dateHistoRes.Buckets[0].KeyAsString != "2011-01-01T00:00:00.000Z" {
		t.Errorf("expected %q; got: %q", "2011-01-01T00:00:00.000Z", *dateHistoRes.Buckets[0].KeyAsString)
	}
	if dateHistoRes.Buckets[1].DocCount != 2 {
		t.Errorf("expected %d; got: %d", 2, dateHistoRes.Buckets[1].DocCount)
	}
	if dateHistoRes.Buckets[1].Key != 1.325376e+12 {
		t.Errorf("expected %v; got: %v", 1.325376e+12, dateHistoRes.Buckets[1].Key)
	}
	if dateHistoRes.Buckets[1].KeyAsString == nil {
		t.Fatalf("expected != nil; got: %v", dateHistoRes.Buckets[1].KeyAsString)
	}
	if *dateHistoRes.Buckets[1].KeyAsString != "2012-01-01T00:00:00.000Z" {
		t.Errorf("expected %q; got: %q", "2012-01-01T00:00:00.000Z", *dateHistoRes.Buckets[1].KeyAsString)
	}

	// dateHistoKeyed
	{
		res, found := agg.KeyedDateHistogram("dateHistoKeyed")
		if !found {
			t.Errorf("expected %v; got: %v", true, found)
		}
		if res == nil {
			t.Fatalf("expected != nil; got: nil")
		}
		if len(res.Buckets) != 2 {
			t.Fatalf("expected %d; got: %d", 2, len(res.Buckets))
		}

		bucket, ok := res.Buckets["2011-01-01T00:00:00.000Z"]
		if !ok || bucket == nil {
			t.Fatalf("expected to have bucket with key %q", "2011-01-01T00:00:00.000Z")
		}
		if bucket.DocCount != 1 {
			t.Errorf("expected %d; got: %d", 1, bucket.DocCount)
		}
		if bucket.Key != 1.29384e+12 {
			t.Errorf("expected %v; got: %v", 1.29384e+12, bucket.Key)
		}
		if bucket.KeyAsString == nil {
			t.Fatalf("expected != nil; got: %v", bucket.KeyAsString)
		}
		if *bucket.KeyAsString != "2011-01-01T00:00:00.000Z" {
			t.Errorf("expected %q; got: %q", "2011-01-01T00:00:00.000Z", *bucket.KeyAsString)
		}

		bucket, ok = res.Buckets["2012-01-01T00:00:00.000Z"]
		if !ok || bucket == nil {
			t.Fatalf("expected to have bucket with key %q", "2012-01-01T00:00:00.000Z")
		}
		if bucket.DocCount != 2 {
			t.Errorf("expected %d; got: %d", 2, bucket.DocCount)
		}
		if bucket.Key != 1.325376e+12 {
			t.Errorf("expected %v; got: %v", 1.325376e+12, bucket.Key)
		}
		if bucket.KeyAsString == nil {
			t.Fatalf("expected != nil; got: %v", bucket.KeyAsString)
		}
		if *bucket.KeyAsString != "2012-01-01T00:00:00.000Z" {
			t.Errorf("expected %q; got: %q", "2012-01-01T00:00:00.000Z", *bucket.KeyAsString)
		}
	}

	// topHits
	topTags, found := agg.Terms("top-tags")
	if !found {
		t.Errorf("expected %v; got: %v", true, found)
	}
	if topTags == nil {
		t.Fatalf("expected != nil; got: nil")
	}
	if topTags.DocCountErrorUpperBound != 0 {
		t.Errorf("expected %v; got: %v", 0, topTags.DocCountErrorUpperBound)
	}
	if topTags.SumOfOtherDocCount != 1 {
		t.Errorf("expected %v; got: %v", 1, topTags.SumOfOtherDocCount)
	}
	if len(topTags.Buckets) != 3 {
		t.Fatalf("expected %d; got: %d", 3, len(topTags.Buckets))
	}
	if topTags.Buckets[0].DocCount != 2 {
		t.Errorf("expected %d; got: %d", 2, topTags.Buckets[0].DocCount)
	}
	if topTags.Buckets[0].Key != "golang" {
		t.Errorf("expected %v; got: %v", "golang", topTags.Buckets[0].Key)
	}
	topHits, found := topTags.Buckets[0].TopHits("top_tag_hits")
	if !found {
		t.Errorf("expected %v; got: %v", true, found)
	}
	if topHits == nil {
		t.Fatal("expected != nil; got: nil")
	}
	if topHits.Hits == nil {
		t.Fatalf("expected != nil; got: nil")
	}
	if topHits.Hits.TotalHits == nil {
		t.Fatalf("expected != nil; got: nil")
	}
	if topHits.Hits.TotalHits.Value != 2 {
		t.Errorf("expected %d; got: %d", 2, topHits.Hits.TotalHits.Value)
	}
	if topHits.Hits.Hits == nil {
		t.Fatalf("expected != nil; got: nil")
	}
	if len(topHits.Hits.Hits) != 2 {
		t.Fatalf("expected %d; got: %d", 2, len(topHits.Hits.Hits))
	}
	hit := topHits.Hits.Hits[0]
	if !found {
		t.Fatalf("expected %v; got: %v", true, found)
	}
	if hit == nil {
		t.Fatal("expected != nil; got: nil")
	}
	var tw tweet
	if err := json.Unmarshal(hit.Source, &tw); err != nil {
		t.Fatalf("expected no error; got: %v", err)
	}
	if tw.Message != "Welcome to Golang and Elasticsearch." {
		t.Errorf("expected %q; got: %q", "Welcome to Golang and Elasticsearch.", tw.Message)
	}
	if topTags.Buckets[1].DocCount != 1 {
		t.Errorf("expected %d; got: %d", 1, topTags.Buckets[1].DocCount)
	}
	if topTags.Buckets[1].Key != "cycling" {
		t.Errorf("expected %v; got: %v", "cycling", topTags.Buckets[1].Key)
	}
	topHits, found = topTags.Buckets[1].TopHits("top_tag_hits")
	if !found {
		t.Errorf("expected %v; got: %v", true, found)
	}
	if topHits == nil {
		t.Fatal("expected != nil; got: nil")
	}
	if topHits.Hits == nil {
		t.Fatal("expected != nil; got nil")
	}
	if topHits.Hits.TotalHits == nil {
		t.Fatal("expected != nil; got nil")
	}
	if topHits.Hits.TotalHits.Value != 1 {
		t.Errorf("expected %d; got: %d", 1, topHits.Hits.TotalHits.Value)
	}
	if topTags.Buckets[2].DocCount != 1 {
		t.Errorf("expected %d; got: %d", 1, topTags.Buckets[2].DocCount)
	}
	if topTags.Buckets[2].Key != "elasticsearch" {
		t.Errorf("expected %v; got: %v", "elasticsearch", topTags.Buckets[2].Key)
	}
	topHits, found = topTags.Buckets[2].TopHits("top_tag_hits")
	if !found {
		t.Errorf("expected %v; got: %v", true, found)
	}
	if topHits == nil {
		t.Fatal("expected != nil; got: nil")
	}
	if topHits.Hits == nil {
		t.Fatal("expected != nil; got: nil")
	}
	if topHits.Hits.TotalHits == nil {
		t.Fatal("expected != nil; got: nil")
	}
	if topHits.Hits.TotalHits.Value != 1 {
		t.Errorf("expected %d; got: %d", 1, topHits.Hits.TotalHits.Value)
	}

	// viewport via geo_bounds (1.3.0 has an error in that it doesn't output the aggregation name)
	geoBoundsRes, found := agg.GeoBounds("viewport")
	if !found {
		t.Errorf("expected %v; got: %v", true, found)
	}
	if geoBoundsRes == nil {
		t.Fatalf("expected != nil; got: nil")
	}

	// geohashed via geohash
	geoHashRes, found := agg.GeoHash("geohashed")
	if !found {
		t.Errorf("expected %v; got: %v", true, found)
	}
	if geoHashRes == nil {
		t.Fatalf("expected != nil; got: nil")
	}

	// geo_centroid
	geoCentroidRes, found := agg.GeoCentroid("centroid")
	if !found {
		t.Errorf("expected %v; got: %v", true, found)
	}
	if geoCentroidRes == nil {
		t.Fatalf("expected != nil; got: nil")
	}

	// Filters agg "countByUser" (unnamed)
	countByUserAggRes, found := agg.Filters("countByUser")
	if !found {
		t.Errorf("expected %v; got: %v", true, found)
	}
	if countByUserAggRes == nil {
		t.Fatalf("expected != nil; got: nil")
	}
	if len(countByUserAggRes.Buckets) != 2 {
		t.Fatalf("expected %d; got: %d", 2, len(countByUserAggRes.Buckets))
	}
	if len(countByUserAggRes.NamedBuckets) != 0 {
		t.Fatalf("expected %d; got: %d", 0, len(countByUserAggRes.NamedBuckets))
	}
	if countByUserAggRes.Buckets[0].DocCount != 2 {
		t.Errorf("expected %d; got: %d", 2, countByUserAggRes.Buckets[0].DocCount)
	}
	if countByUserAggRes.Buckets[1].DocCount != 1 {
		t.Errorf("expected %d; got: %d", 1, countByUserAggRes.Buckets[1].DocCount)
	}

	// Filters agg "countByUser2" (named)
	countByUser2AggRes, found := agg.Filters("countByUser2")
	if !found {
		t.Errorf("expected %v; got: %v", true, found)
	}
	if countByUser2AggRes == nil {
		t.Fatalf("expected != nil; got: nil")
	}
	if len(countByUser2AggRes.Buckets) != 0 {
		t.Fatalf("expected %d; got: %d", 0, len(countByUser2AggRes.Buckets))
	}
	if len(countByUser2AggRes.NamedBuckets) != 2 {
		t.Fatalf("expected %d; got: %d", 2, len(countByUser2AggRes.NamedBuckets))
	}
	b, found := countByUser2AggRes.NamedBuckets["olivere"]
	if !found {
		t.Fatalf("expected bucket %q; got: %v", "olivere", found)
	}
	if b == nil {
		t.Fatalf("expected bucket %q; got: %v", "olivere", b)
	}
	if b.DocCount != 2 {
		t.Errorf("expected %d; got: %d", 2, b.DocCount)
	}
	b, found = countByUser2AggRes.NamedBuckets["sandrae"]
	if !found {
		t.Fatalf("expected bucket %q; got: %v", "sandrae", found)
	}
	if b == nil {
		t.Fatalf("expected bucket %q; got: %v", "sandrae", b)
	}
	if b.DocCount != 1 {
		t.Errorf("expected %d; got: %d", 1, b.DocCount)
	}

	// AdjacencyMatrix agg "adjacencyMatrixAgg" (named)
	adjacencyMatrixAggRes, found := agg.AdjacencyMatrix("interactions")
	if !found {
		t.Errorf("expected %v; got: %v", true, found)
	}
	if adjacencyMatrixAggRes == nil {
		t.Fatalf("expected != nil; got: nil")
	}
	if len(adjacencyMatrixAggRes.Buckets) != 2 {
		t.Fatalf("expected %d; got: %d", 2, len(adjacencyMatrixAggRes.Buckets))
	}
	if adjacencyMatrixAggRes.Buckets[0].DocCount != 2 {
		t.Errorf("expected %d; got: %d", 2, adjacencyMatrixAggRes.Buckets[0].DocCount)
	}
	if adjacencyMatrixAggRes.Buckets[1].DocCount != 1 {
		t.Errorf("expected %d; got: %d", 1, adjacencyMatrixAggRes.Buckets[1].DocCount)
	}

	// movingAvgDateHisto
	{
		movingAvgDateHistoRes, found := agg.DateHistogram("movingAvgDateHisto")
		if !found {
			t.Fatalf("expected %v; got: %v", true, false)
		}
		if movingAvgDateHistoRes == nil {
			t.Fatal("expected != nil; got: nil")
		}
		if want, have := 2, len(movingAvgDateHistoRes.Buckets); want != have {
			t.Fatalf("expected %d buckets, have %d", want, have)
		}
		// movingAvgDateHisto.Buckets[0]
		if want, have := int64(1), movingAvgDateHistoRes.Buckets[0].DocCount; want != have {
			t.Fatalf("expected %d docs in bucket 0, have %d", want, have)
		}
		if want, have := 1293840000000.0, movingAvgDateHistoRes.Buckets[0].Key; want != have {
			t.Fatalf("expected key of %v in bucket 0, have %v", want, have)
		}
		if have := movingAvgDateHistoRes.Buckets[0].KeyAsString; have == nil {
			t.Fatalf("expected key_as_string != nil in bucket 0, have %v", have)
		}
		if want, have := "2011-01-01T00:00:00.000Z", *movingAvgDateHistoRes.Buckets[0].KeyAsString; want != have {
			t.Fatalf("expected key_as_string of %q in bucket 0, have %q", want, have)
		}
		sumOfRetweetsAgg, found := movingAvgDateHistoRes.Buckets[0].SumBucket("sumOfRetweets")
		if !found {
			t.Fatalf("expected sub-aggregation %q", "sumOfRetweets")
		}
		if have := sumOfRetweetsAgg.Value; have == nil {
			t.Fatalf("expected sumOfRetweets != nil, have %v", have)
		}
		if want, have := 12.0, *sumOfRetweetsAgg.Value; want != have {
			t.Fatalf("expected sumOfRetweets = %v, have %v", want, have)
		}
		movingAvgAgg, found := movingAvgDateHistoRes.Buckets[0].MovAvg("movingAvg")
		if found {
			t.Fatalf("expected no sub-aggregation %q", "movingAvg")
		}
		if movingAvgAgg != nil {
			t.Fatalf("expected no sub-aggregation %q", "movingAvg")
		}
		movingFnAgg, found := movingAvgDateHistoRes.Buckets[0].MovFn("movingFn")
		if !found {
			t.Fatalf("expected sub-aggregation %q", "movingFn")
		}
		if have := movingFnAgg.Value; have == nil {
			t.Fatalf("expected movingFn != nil, have %v", have)
		}
		if want, have := 0.0, *movingFnAgg.Value; want != have {
			t.Fatalf("expected movingFn = %v, have %v", want, have)
		}
		// movingAvgDateHisto.Buckets[1]
		if want, have := int64(2), movingAvgDateHistoRes.Buckets[1].DocCount; want != have {
			t.Fatalf("expected %d docs in bucket 1, have %d", want, have)
		}
		if want, have := 1325376000000.0, movingAvgDateHistoRes.Buckets[1].Key; want != have {
			t.Fatalf("expected key of %v in bucket 1, have %v", want, have)
		}
		if have := movingAvgDateHistoRes.Buckets[1].KeyAsString; have == nil {
			t.Fatalf("expected key_as_string != nil in bucket 1, have %v", have)
		}
		if want, have := "2012-01-01T00:00:00.000Z", *movingAvgDateHistoRes.Buckets[1].KeyAsString; want != have {
			t.Fatalf("expected key_as_string of %q in bucket 1, have %q", want, have)
		}
		sumOfRetweetsAgg, found = movingAvgDateHistoRes.Buckets[1].SumBucket("sumOfRetweets")
		if !found {
			t.Fatalf("expected sub-aggregation %q", "sumOfRetweets")
		}
		if have := sumOfRetweetsAgg.Value; have == nil {
			t.Fatalf("expected sumOfRetweets != nil, have %v", have)
		}
		if want, have := 108.0, *sumOfRetweetsAgg.Value; want != have {
			t.Fatalf("expected sumOfRetweets = %v, have %v", want, have)
		}
		movingAvgAgg, found = movingAvgDateHistoRes.Buckets[1].MovAvg("movingAvg")
		if !found {
			t.Fatalf("expected sub-aggregation %q", "movingAvg")
		}
		if have := movingAvgAgg.Value; have == nil {
			t.Fatalf("expected movingAgg != nil, have %v", have)
		}
		if want, have := 12.0, *movingAvgAgg.Value; want != have {
			t.Fatalf("expected movingAvg = %v, have %v", want, have)
		}
		movingFnAgg, found = movingAvgDateHistoRes.Buckets[1].MovFn("movingFn")
		if !found {
			t.Fatalf("expected sub-aggregation %q", "movingFn")
		}
		if have := movingFnAgg.Value; have == nil {
			t.Fatalf("expected movingFn != nil, have %v", have)
		}
		if want, have := 12.0, *movingFnAgg.Value; want != have {
			t.Fatalf("expected movingFn = %v, have %v", want, have)
		}
	}
}

// TestAggsCompositeIntegration is an integration test for the Composite aggregation.
func TestAggsCompositeIntegration(t *testing.T) {
	// client := setupTestClientAndCreateIndex(t, SetTraceLog(log.New(os.Stdout, "", log.LstdFlags)))
	client := setupTestClientAndCreateIndex(t)

	tweet1 := tweet{
		User:     "olivere",
		Retweets: 108,
		Message:  "Welcome to Golang and Elasticsearch.",
		Image:    "http://golang.org/doc/gopher/gophercolor.png",
		Tags:     []string{"golang", "elasticsearch"},
		Location: "48.1333,11.5667", // lat,lon
		Created:  time.Date(2012, 12, 12, 17, 38, 34, 0, time.UTC),
	}
	tweet2 := tweet{
		User:     "olivere",
		Retweets: 0,
		Message:  "Another unrelated topic.",
		Tags:     []string{"golang"},
		Location: "48.1189,11.4289", // lat,lon
		Created:  time.Date(2012, 10, 10, 8, 12, 03, 0, time.UTC),
	}
	tweet3 := tweet{
		User:     "sandrae",
		Retweets: 12,
		Message:  "Cycling is fun.",
		Tags:     []string{"sports", "cycling"},
		Location: "47.7167,11.7167", // lat,lon
		Created:  time.Date(2011, 11, 11, 10, 58, 12, 0, time.UTC),
	}

	// Add all documents
	_, err := client.Index().Index(testIndexName).Id("1").BodyJson(&tweet1).Do(context.TODO())
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.Index().Index(testIndexName).Id("2").BodyJson(&tweet2).Do(context.TODO())
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.Index().Index(testIndexName).Id("3").BodyJson(&tweet3).Do(context.TODO())
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.Refresh().Index(testIndexName).Do(context.TODO())
	if err != nil {
		t.Fatal(err)
	}

	count, err := client.Count(testIndexName).Do(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if want, have := int64(3), count; want != have {
		t.Fatalf("expected %d documents, got %d", want, have)
	}

	// Match all should return all documents
	all := NewMatchAllQuery()

	// Terms Aggregate by user name
	builder := client.Search().Index(testIndexName).Query(all).Pretty(true)
	composite := NewCompositeAggregation().Sources(
		NewCompositeAggregationTermsValuesSource("composite_users").Field("user"),
		NewCompositeAggregationHistogramValuesSource("composite_retweets", 1).MissingBucket(true).Field("retweets"),
		NewCompositeAggregationDateHistogramValuesSource("composite_created", "1m").Field("created"),
	).Size(2)
	builder = builder.Aggregation("composite", composite)

	// Run the query
	searchResult, err := builder.Pretty(true).Do(context.TODO())
	if err != nil {
		t.Fatal(err)
	}
	if searchResult.Hits == nil {
		t.Errorf("expected Hits != nil; got: nil")
	}
	if searchResult.Hits.TotalHits == nil {
		t.Errorf("expected Hits.TotalHits != nil; got: nil")
	}
	if searchResult.Hits.TotalHits.Value != 3 {
		t.Errorf("expected Hits.TotalHits.Value = %d; got: %d", 3, searchResult.Hits.TotalHits.Value)
	}
	if len(searchResult.Hits.Hits) != 3 {
		t.Errorf("expected len(Hits.Hits) = %d; got: %d", 3, len(searchResult.Hits.Hits))
	}
	agg := searchResult.Aggregations
	if agg == nil {
		t.Fatalf("expected Aggregations != nil; got: nil")
	}

	// Check outcome of 1st call (without "after_key" settings)
	var afterKey map[string]interface{}
	{
		compositeAggRes, found := agg.Composite("composite")
		if !found {
			t.Errorf("expected %v; got: %v", true, found)
		}
		if compositeAggRes == nil {
			t.Fatalf("expected != nil; got: nil")
		}
		if want, have := 2, len(compositeAggRes.Buckets); want != have {
			t.Fatalf("expected %d; got: %d", want, have)
		}
		afterKey = compositeAggRes.AfterKey
		if afterKey == nil || len(afterKey) == 0 {
			t.Fatalf("expected after_key; got: %v", afterKey)
		}
		if v, found := afterKey["composite_users"]; !found {
			t.Fatalf("expected after_key.composite_users; got: %v", afterKey)
		} else if want, have := "olivere", v; want != have {
			t.Fatalf("expected after_key.composite_users = %q; got: %q", want, have)
		}
		if v, found := afterKey["composite_retweets"]; !found {
			t.Fatalf("expected after_key.composite_retweets; got: %v", afterKey)
		} else if want, have := 108.0, v; want != have {
			t.Fatalf("expected after_key.composite_retweets = %v; got: %v", want, have)
		}
		if v, found := afterKey["composite_created"]; !found {
			t.Fatalf("expected after_key.composite_created; got: %v", afterKey)
		} else if want, have := 1355333880000.0, v; want != have {
			t.Fatalf("expected after_key.composite_created = %v; got: %v", want, have)
		}
	}

	// Now paginate to the 2nd call via "after_key"
	builder = client.Search().Index(testIndexName).Query(all).Pretty(true)
	composite = NewCompositeAggregation().Sources(
		NewCompositeAggregationTermsValuesSource("composite_users").Field("user"),
		NewCompositeAggregationHistogramValuesSource("composite_retweets", 1).Field("retweets"),
		NewCompositeAggregationDateHistogramValuesSource("composite_created", "1m").Field("created"),
	).Size(2).AggregateAfter(afterKey)
	builder = builder.Aggregation("composite", composite)
	searchResult, err = builder.Pretty(true).Do(context.TODO())
	if err != nil {
		t.Fatal(err)
	}
	if searchResult.Hits == nil {
		t.Errorf("expected Hits != nil; got: nil")
	}
	agg = searchResult.Aggregations
	if agg == nil {
		t.Fatalf("expected Aggregations != nil; got: nil")
	}

	// Check outcome of 2nd call (with "after_key" settings)
	{
		compositeAggRes, found := agg.Composite("composite")
		if !found {
			t.Errorf("expected %v; got: %v", true, found)
		}
		if compositeAggRes == nil {
			t.Fatalf("expected != nil; got: nil")
		}
		if want, have := 1, len(compositeAggRes.Buckets); want != have {
			t.Fatalf("expected %d; got: %d", want, have)
		}
		afterKey = compositeAggRes.AfterKey
		if afterKey == nil || len(afterKey) == 0 {
			t.Fatalf("expected after_key; got: %v", afterKey)
		}
		if v, found := afterKey["composite_users"]; !found {
			t.Fatalf("expected after_key.composite_users; got: %v", afterKey)
		} else if want, have := "sandrae", v; want != have {
			t.Fatalf("expected after_key.composite_users = %q; got: %q", want, have)
		}
		if v, found := afterKey["composite_retweets"]; !found {
			t.Fatalf("expected after_key.composite_retweets; got: %v", afterKey)
		} else if want, have := 12.0, v; want != have {
			t.Fatalf("expected after_key.composite_retweets = %v; got: %v", want, have)
		}
		if v, found := afterKey["composite_created"]; !found {
			t.Fatalf("expected after_key.composite_created; got: %v", afterKey)
		} else if want, have := 1321009080000.0, v; want != have {
			t.Fatalf("expected after_key.composite_created = %v; got: %v", want, have)
		}
	}

	// Now paginate to the 3rd call via "after_key"
	builder = client.Search().Index(testIndexName).Query(all).Pretty(true)
	composite = NewCompositeAggregation().Sources(
		NewCompositeAggregationTermsValuesSource("composite_users").Field("user"),
		NewCompositeAggregationHistogramValuesSource("composite_retweets", 1).Field("retweets"),
		NewCompositeAggregationDateHistogramValuesSource("composite_created", "1m").Field("created"),
	).Size(2).AggregateAfter(afterKey)
	builder = builder.Aggregation("composite", composite)
	searchResult, err = builder.Pretty(true).Do(context.TODO())
	if err != nil {
		t.Fatal(err)
	}
	if searchResult.Hits == nil {
		t.Errorf("expected Hits != nil; got: nil")
	}
	agg = searchResult.Aggregations
	if agg == nil {
		t.Fatalf("expected Aggregations != nil; got: nil")
	}

	// Check outcome of 3rd call (with "after_key" settings)
	{
		compositeAggRes, found := agg.Composite("composite")
		if !found {
			t.Errorf("expected %v; got: %v", true, found)
		}
		if compositeAggRes == nil {
			t.Fatalf("expected != nil; got: nil")
		}
		if want, have := 0, len(compositeAggRes.Buckets); want != have {
			t.Fatalf("expected %d; got: %d", want, have)
		}
		afterKey = compositeAggRes.AfterKey
		if afterKey != nil {
			t.Fatalf("expected no after_key; got: %v", afterKey)
		}
	}
}

// TestAggsMarshal ensures that marshaling aggregations back into a string
// does not yield base64 encoded data. See https://github.com/facert/elastic/issues/51
// and https://groups.google.com/forum/#!topic/Golang-Nuts/38ShOlhxAYY for details.
func TestAggsMarshal(t *testing.T) {
	client := setupTestClientAndCreateIndex(t)

	tweet1 := tweet{
		User:     "olivere",
		Retweets: 108,
		Message:  "Welcome to Golang and Elasticsearch.",
		Image:    "http://golang.org/doc/gopher/gophercolor.png",
		Tags:     []string{"golang", "elasticsearch"},
		Location: "48.1333,11.5667", // lat,lon
		Created:  time.Date(2012, 12, 12, 17, 38, 34, 0, time.UTC),
	}

	// Add all documents
	_, err := client.Index().Index(testIndexName).Id("1").BodyJson(&tweet1).Do(context.TODO())
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Refresh().Index(testIndexName).Do(context.TODO())
	if err != nil {
		t.Fatal(err)
	}

	// Match all should return all documents
	all := NewMatchAllQuery()
	dhagg := NewDateHistogramAggregation().Field("created").Interval("year")

	// Run query
	builder := client.Search().Index(testIndexName).Query(all)
	builder = builder.Aggregation("dhagg", dhagg)
	searchResult, err := builder.Do(context.TODO())
	if err != nil {
		t.Fatal(err)
	}
	if searchResult.TotalHits() != 1 {
		t.Errorf("expected Hits.TotalHits = %d; got: %d", 1, searchResult.TotalHits())
	}
	if _, found := searchResult.Aggregations["dhagg"]; !found {
		t.Fatalf("expected aggregation %q", "dhagg")
	}
	buf, err := json.Marshal(searchResult)
	if err != nil {
		t.Fatal(err)
	}
	s := string(buf)
	if i := strings.Index(s, `{"dhagg":{"buckets":[{"key_as_string":"2012-01-01`); i < 0 {
		t.Errorf("expected to serialize aggregation into string; got: %v", s)
	}
}

func TestAggsMetricsMin(t *testing.T) {
	s := `{
	"min_price": {
  	"value": 10
  }
}`

	aggs := new(Aggregations)
	err := json.Unmarshal([]byte(s), &aggs)
	if err != nil {
		t.Fatalf("expected no error decoding; got: %v", err)
	}

	agg, found := aggs.Min("min_price")
	if !found {
		t.Fatalf("expected aggregation to be found; got: %v", found)
	}
	if agg == nil {
		t.Fatalf("expected aggregation != nil; got: %v", agg)
	}
	if agg.Value == nil {
		t.Fatalf("expected aggregation value != nil; got: %v", agg.Value)
	}
	if *agg.Value != float64(10) {
		t.Fatalf("expected aggregation value = %v; got: %v", float64(10), *agg.Value)
	}
}

func TestAggsMetricsMax(t *testing.T) {
	s := `{
	"max_price": {
  	"value": 35
  }
}`

	aggs := new(Aggregations)
	err := json.Unmarshal([]byte(s), &aggs)
	if err != nil {
		t.Fatalf("expected no error decoding; got: %v", err)
	}

	agg, found := aggs.Max("max_price")
	if !found {
		t.Fatalf("expected aggregation to be found; got: %v", found)
	}
	if agg == nil {
		t.Fatalf("expected aggregation != nil; got: %v", agg)
	}
	if agg.Value == nil {
		t.Fatalf("expected aggregation value != nil; got: %v", agg.Value)
	}
	if *agg.Value != float64(35) {
		t.Fatalf("expected aggregation value = %v; got: %v", float64(35), *agg.Value)
	}
}

func TestAggsMetricsSum(t *testing.T) {
	s := `{
	"intraday_return": {
  	"value": 2.18
  }
}`

	aggs := new(Aggregations)
	err := json.Unmarshal([]byte(s), &aggs)
	if err != nil {
		t.Fatalf("expected no error decoding; got: %v", err)
	}

	agg, found := aggs.Sum("intraday_return")
	if !found {
		t.Fatalf("expected aggregation to be found; got: %v", found)
	}
	if agg == nil {
		t.Fatalf("expected aggregation != nil; got: %v", agg)
	}
	if agg.Value == nil {
		t.Fatalf("expected aggregation value != nil; got: %v", agg.Value)
	}
	if *agg.Value != float64(2.18) {
		t.Fatalf("expected aggregation value = %v; got: %v", float64(2.18), *agg.Value)
	}
}

func TestAggsMetricsAvg(t *testing.T) {
	s := `{
	"avg_grade": {
  	"value": 75
  }
}`

	aggs := new(Aggregations)
	err := json.Unmarshal([]byte(s), &aggs)
	if err != nil {
		t.Fatalf("expected no error decoding; got: %v", err)
	}

	agg, found := aggs.Avg("avg_grade")
	if !found {
		t.Fatalf("expected aggregation to be found; got: %v", found)
	}
	if agg == nil {
		t.Fatalf("expected aggregation != nil; got: %v", agg)
	}
	if agg.Value == nil {
		t.Fatalf("expected aggregation value != nil; got: %v", agg.Value)
	}
	if *agg.Value != float64(75) {
		t.Fatalf("expected aggregation value = %v; got: %v", float64(75), *agg.Value)
	}
}

func TestAggsMetricsValueCount(t *testing.T) {
	s := `{
	"grades_count": {
  	"value": 10
  }
}`

	aggs := new(Aggregations)
	err := json.Unmarshal([]byte(s), &aggs)
	if err != nil {
		t.Fatalf("expected no error decoding; got: %v", err)
	}

	agg, found := aggs.ValueCount("grades_count")
	if !found {
		t.Fatalf("expected aggregation to be found; got: %v", found)
	}
	if agg == nil {
		t.Fatalf("expected aggregation != nil; got: %v", agg)
	}
	if agg.Value == nil {
		t.Fatalf("expected aggregation value != nil; got: %v", agg.Value)
	}
	if *agg.Value != float64(10) {
		t.Fatalf("expected aggregation value = %v; got: %v", float64(10), *agg.Value)
	}
}

func TestAggsMetricsCardinality(t *testing.T) {
	s := `{
	"author_count": {
  	"value": 12
  }
}`

	aggs := new(Aggregations)
	err := json.Unmarshal([]byte(s), &aggs)
	if err != nil {
		t.Fatalf("expected no error decoding; got: %v", err)
	}

	agg, found := aggs.Cardinality("author_count")
	if !found {
		t.Fatalf("expected aggregation to be found; got: %v", found)
	}
	if agg == nil {
		t.Fatalf("expected aggregation != nil; got: %v", agg)
	}
	if agg.Value == nil {
		t.Fatalf("expected aggregation value != nil; got: %v", agg.Value)
	}
	if *agg.Value != float64(12) {
		t.Fatalf("expected aggregation value = %v; got: %v", float64(12), *agg.Value)
	}
}

func TestAggsMetricsStats(t *testing.T) {
	s := `{
	"grades_stats": {
    "count": 6,
    "min": 60,
    "max": 98,
    "avg": 78.5,
    "sum": 471
  }
}`

	aggs := new(Aggregations)
	err := json.Unmarshal([]byte(s), &aggs)
	if err != nil {
		t.Fatalf("expected no error decoding; got: %v", err)
	}

	agg, found := aggs.Stats("grades_stats")
	if !found {
		t.Fatalf("expected aggregation to be found; got: %v", found)
	}
	if agg == nil {
		t.Fatalf("expected aggregation != nil; got: %v", agg)
	}
	if agg.Count != int64(6) {
		t.Fatalf("expected aggregation Count = %v; got: %v", int64(6), agg.Count)
	}
	if agg.Min == nil {
		t.Fatalf("expected aggregation Min != nil; got: %v", agg.Min)
	}
	if *agg.Min != float64(60) {
		t.Fatalf("expected aggregation Min = %v; got: %v", float64(60), *agg.Min)
	}
	if agg.Max == nil {
		t.Fatalf("expected aggregation Max != nil; got: %v", agg.Max)
	}
	if *agg.Max != float64(98) {
		t.Fatalf("expected aggregation Max = %v; got: %v", float64(98), *agg.Max)
	}
	if agg.Avg == nil {
		t.Fatalf("expected aggregation Avg != nil; got: %v", agg.Avg)
	}
	if *agg.Avg != float64(78.5) {
		t.Fatalf("expected aggregation Avg = %v; got: %v", float64(78.5), *agg.Avg)
	}
	if agg.Sum == nil {
		t.Fatalf("expected aggregation Sum != nil; got: %v", agg.Sum)
	}
	if *agg.Sum != float64(471) {
		t.Fatalf("expected aggregation Sum = %v; got: %v", float64(471), *agg.Sum)
	}
}

func TestAggsMetricsExtendedStats(t *testing.T) {
	s := `{
	"grades_stats": {
    "count": 6,
    "min": 72,
    "max": 117.6,
    "avg": 94.2,
    "sum": 565.2,
    "sum_of_squares": 54551.51999999999,
    "variance": 218.2799999999976,
    "std_deviation": 14.774302013969987
  }
}`

	aggs := new(Aggregations)
	err := json.Unmarshal([]byte(s), &aggs)
	if err != nil {
		t.Fatalf("expected no error decoding; got: %v", err)
	}

	agg, found := aggs.ExtendedStats("grades_stats")
	if !found {
		t.Fatalf("expected aggregation to be found; got: %v", found)
	}
	if agg == nil {
		t.Fatalf("expected aggregation != nil; got: %v", agg)
	}
	if agg.Count != int64(6) {
		t.Fatalf("expected aggregation Count = %v; got: %v", int64(6), agg.Count)
	}
	if agg.Min == nil {
		t.Fatalf("expected aggregation Min != nil; got: %v", agg.Min)
	}
	if *agg.Min != float64(72) {
		t.Fatalf("expected aggregation Min = %v; got: %v", float64(72), *agg.Min)
	}
	if agg.Max == nil {
		t.Fatalf("expected aggregation Max != nil; got: %v", agg.Max)
	}
	if *agg.Max != float64(117.6) {
		t.Fatalf("expected aggregation Max = %v; got: %v", float64(117.6), *agg.Max)
	}
	if agg.Avg == nil {
		t.Fatalf("expected aggregation Avg != nil; got: %v", agg.Avg)
	}
	if *agg.Avg != float64(94.2) {
		t.Fatalf("expected aggregation Avg = %v; got: %v", float64(94.2), *agg.Avg)
	}
	if agg.Sum == nil {
		t.Fatalf("expected aggregation Sum != nil; got: %v", agg.Sum)
	}
	if *agg.Sum != float64(565.2) {
		t.Fatalf("expected aggregation Sum = %v; got: %v", float64(565.2), *agg.Sum)
	}
	if agg.SumOfSquares == nil {
		t.Fatalf("expected aggregation sum_of_squares != nil; got: %v", agg.SumOfSquares)
	}
	if *agg.SumOfSquares != float64(54551.51999999999) {
		t.Fatalf("expected aggregation sum_of_squares = %v; got: %v", float64(54551.51999999999), *agg.SumOfSquares)
	}
	if agg.Variance == nil {
		t.Fatalf("expected aggregation Variance != nil; got: %v", agg.Variance)
	}
	if *agg.Variance != float64(218.2799999999976) {
		t.Fatalf("expected aggregation Variance = %v; got: %v", float64(218.2799999999976), *agg.Variance)
	}
	if agg.StdDeviation == nil {
		t.Fatalf("expected aggregation StdDeviation != nil; got: %v", agg.StdDeviation)
	}
	if *agg.StdDeviation != float64(14.774302013969987) {
		t.Fatalf("expected aggregation StdDeviation = %v; got: %v", float64(14.774302013969987), *agg.StdDeviation)
	}
}

func TestAggsMatrixStats(t *testing.T) {
	s := `{
	"matrixstats": {
		"fields": [{
			"name": "income",
			"count": 50,
			"mean": 51985.1,
			"variance": 7.383377037755103E7,
			"skewness": 0.5595114003506483,
			"kurtosis": 2.5692365287787124,
			"covariance": {
				"income": 7.383377037755103E7,
				"poverty": -21093.65836734694
			},
			"correlation": {
				"income": 1.0,
				"poverty": -0.8352655256272504
			}
		}, {
			"name": "poverty",
			"count": 51,
			"mean": 12.732000000000001,
			"variance": 8.637730612244896,
			"skewness": 0.4516049811903419,
			"kurtosis": 2.8615929677997767,
			"covariance": {
				"income": -21093.65836734694,
				"poverty": 8.637730612244896
			},
			"correlation": {
				"income": -0.8352655256272504,
				"poverty": 1.0
			}
		}]
	}
}`

	aggs := new(Aggregations)
	err := json.Unmarshal([]byte(s), &aggs)
	if err != nil {
		t.Fatalf("expected no error decoding; got: %v", err)
	}

	agg, found := aggs.MatrixStats("matrixstats")
	if !found {
		t.Fatalf("expected aggregation to be found; got: %v", found)
	}
	if agg == nil {
		t.Fatalf("expected aggregation != nil; got: %v", agg)
	}
	if want, got := 2, len(agg.Fields); want != got {
		t.Fatalf("expected aggregaton len(Fields) = %v; got: %v", want, got)
	}
	field := agg.Fields[0]
	if want, got := "income", field.Name; want != got {
		t.Fatalf("expected aggregation field name == %q; got: %q", want, got)
	}
	if want, got := int64(50), field.Count; want != got {
		t.Fatalf("expected aggregation field count == %v; got: %v", want, got)
	}
	if want, got := 51985.1, field.Mean; want != got {
		t.Fatalf("expected aggregation field mean == %v; got: %v", want, got)
	}
	if want, got := 7.383377037755103e7, field.Variance; want != got {
		t.Fatalf("expected aggregation field variance == %v; got: %v", want, got)
	}
	if want, got := 0.5595114003506483, field.Skewness; want != got {
		t.Fatalf("expected aggregation field skewness == %v; got: %v", want, got)
	}
	if want, got := 2.5692365287787124, field.Kurtosis; want != got {
		t.Fatalf("expected aggregation field kurtosis == %v; got: %v", want, got)
	}
	if field.Covariance == nil {
		t.Fatalf("expected aggregation field covariance != nil; got: %v", nil)
	}
	if want, got := 7.383377037755103e7, field.Covariance["income"]; want != got {
		t.Fatalf("expected aggregation field covariance == %v; got: %v", want, got)
	}
	if want, got := -21093.65836734694, field.Covariance["poverty"]; want != got {
		t.Fatalf("expected aggregation field covariance == %v; got: %v", want, got)
	}
	if field.Correlation == nil {
		t.Fatalf("expected aggregation field correlation != nil; got: %v", nil)
	}
	if want, got := 1.0, field.Correlation["income"]; want != got {
		t.Fatalf("expected aggregation field correlation == %v; got: %v", want, got)
	}
	if want, got := -0.8352655256272504, field.Correlation["poverty"]; want != got {
		t.Fatalf("expected aggregation field correlation == %v; got: %v", want, got)
	}
	field = agg.Fields[1]
	if want, got := "poverty", field.Name; want != got {
		t.Fatalf("expected aggregation field name == %q; got: %q", want, got)
	}
	if want, got := int64(51), field.Count; want != got {
		t.Fatalf("expected aggregation field count == %v; got: %v", want, got)
	}
}

func TestAggsMetricsPercentiles(t *testing.T) {
	s := `{
  "load_time_outlier": {
		"values" : {
		  "1.0": 15,
		  "5.0": 20,
		  "25.0": 23,
		  "50.0": 25,
		  "75.0": 29,
		  "95.0": 60,
		  "99.0": 150
		}
  }
}`

	aggs := new(Aggregations)
	err := json.Unmarshal([]byte(s), &aggs)
	if err != nil {
		t.Fatalf("expected no error decoding; got: %v", err)
	}

	agg, found := aggs.Percentiles("load_time_outlier")
	if !found {
		t.Fatalf("expected aggregation to be found; got: %v", found)
	}
	if agg == nil {
		t.Fatalf("expected aggregation != nil; got: %v", agg)
	}
	if agg.Values == nil {
		t.Fatalf("expected aggregation Values != nil; got: %v", agg.Values)
	}
	if len(agg.Values) != 7 {
		t.Fatalf("expected %d aggregation Values; got: %d", 7, len(agg.Values))
	}
	if agg.Values["1.0"] != float64(15) {
		t.Errorf("expected aggregation value for \"1.0\" = %v; got: %v", float64(15), agg.Values["1.0"])
	}
	if agg.Values["5.0"] != float64(20) {
		t.Errorf("expected aggregation value for \"5.0\" = %v; got: %v", float64(20), agg.Values["5.0"])
	}
	if agg.Values["25.0"] != float64(23) {
		t.Errorf("expected aggregation value for \"25.0\" = %v; got: %v", float64(23), agg.Values["25.0"])
	}
	if agg.Values["50.0"] != float64(25) {
		t.Errorf("expected aggregation value for \"50.0\" = %v; got: %v", float64(25), agg.Values["50.0"])
	}
	if agg.Values["75.0"] != float64(29) {
		t.Errorf("expected aggregation value for \"75.0\" = %v; got: %v", float64(29), agg.Values["75.0"])
	}
	if agg.Values["95.0"] != float64(60) {
		t.Errorf("expected aggregation value for \"95.0\" = %v; got: %v", float64(60), agg.Values["95.0"])
	}
	if agg.Values["99.0"] != float64(150) {
		t.Errorf("expected aggregation value for \"99.0\" = %v; got: %v", float64(150), agg.Values["99.0"])
	}
}

func TestAggsMetricsPercentileRanks(t *testing.T) {
	s := `{
  "load_time_outlier": {
		"values" : {
		  "15": 92,
		  "30": 100
		}
  }
}`

	aggs := new(Aggregations)
	err := json.Unmarshal([]byte(s), &aggs)
	if err != nil {
		t.Fatalf("expected no error decoding; got: %v", err)
	}

	agg, found := aggs.PercentileRanks("load_time_outlier")
	if !found {
		t.Fatalf("expected aggregation to be found; got: %v", found)
	}
	if agg == nil {
		t.Fatalf("expected aggregation != nil; got: %v", agg)
	}
	if agg.Values == nil {
		t.Fatalf("expected aggregation Values != nil; got: %v", agg.Values)
	}
	if len(agg.Values) != 2 {
		t.Fatalf("expected %d aggregation Values; got: %d", 7, len(agg.Values))
	}
	if agg.Values["15"] != float64(92) {
		t.Errorf("expected aggregation value for \"15\" = %v; got: %v", float64(92), agg.Values["15"])
	}
	if agg.Values["30"] != float64(100) {
		t.Errorf("expected aggregation value for \"30\" = %v; got: %v", float64(100), agg.Values["30"])
	}
}

func TestAggsMetricsTopHits(t *testing.T) {
	s := `{
  "top-tags": {
     "buckets": [
        {
           "key": "windows-7",
           "doc_count": 25365,
           "top_tags_hits": {
              "hits": {
                 "total": {
                   "value": 25365,
                   "relation": "eq"
                 },
                 "max_score": 1,
                 "hits": [
                    {
                       "_index": "stack",
                       "_type": "question",
                       "_id": "602679",
                       "_score": 1,
                       "_source": {
                          "title": "Windows port opening"
                       },
                       "sort": [
                          1370143231177
                       ]
                    }
                 ]
              }
           }
        },
        {
           "key": "linux",
           "doc_count": 18342,
           "top_tags_hits": {
              "hits": {
                 "total": {
                   "value": 18342,
                   "relation": "eq"
                 },
                 "max_score": 1,
                 "hits": [
                    {
                       "_index": "stack",
                       "_type": "question",
                       "_id": "602672",
                       "_score": 1,
                       "_source": {
                          "title": "Ubuntu RFID Screensaver lock-unlock"
                       },
                       "sort": [
                          1370143379747
                       ]
                    }
                 ]
              }
           }
        },
        {
           "key": "windows",
           "doc_count": 18119,
           "top_tags_hits": {
              "hits": {
                 "total": {
                   "value": 18119,
                   "relation": "eq"
                 },
                 "max_score": 1,
                 "hits": [
                    {
                       "_index": "stack",
                       "_type": "question",
                       "_id": "602678",
                       "_score": 1,
                       "_source": {
                          "title": "If I change my computers date / time, what could be affected?"
                       },
                       "sort": [
                          1370142868283
                       ]
                    }
                 ]
              }
           }
        }
     ]
  }
}`

	aggs := new(Aggregations)
	err := json.Unmarshal([]byte(s), &aggs)
	if err != nil {
		t.Fatalf("expected no error decoding; got: %v", err)
	}

	agg, found := aggs.Terms("top-tags")
	if !found {
		t.Fatalf("expected aggregation to be found; got: %v", found)
	}
	if agg == nil {
		t.Fatalf("expected aggregation != nil; got: %v", agg)
	}
	if agg.Buckets == nil {
		t.Fatalf("expected aggregation buckets != nil; got: %v", agg.Buckets)
	}
	if len(agg.Buckets) != 3 {
		t.Errorf("expected %d bucket entries; got: %d", 3, len(agg.Buckets))
	}
	if agg.Buckets[0].Key != "windows-7" {
		t.Errorf("expected bucket key = %q; got: %q", "windows-7", agg.Buckets[0].Key)
	}
	if agg.Buckets[1].Key != "linux" {
		t.Errorf("expected bucket key = %q; got: %q", "linux", agg.Buckets[1].Key)
	}
	if agg.Buckets[2].Key != "windows" {
		t.Errorf("expected bucket key = %q; got: %q", "windows", agg.Buckets[2].Key)
	}

	// Sub-aggregation of top-hits
	subAgg, found := agg.Buckets[0].TopHits("top_tags_hits")
	if !found {
		t.Fatalf("expected sub aggregation to be found; got: %v", found)
	}
	if subAgg == nil {
		t.Fatalf("expected sub aggregation != nil; got: %v", subAgg)
	}
	if subAgg.Hits == nil {
		t.Fatalf("expected sub aggregation Hits != nil; got: %v", subAgg.Hits)
	}
	if subAgg.Hits.TotalHits == nil {
		t.Fatalf("expected sub aggregation Hits.TotalHits != nil; got: %v", subAgg.Hits.TotalHits)
	}
	if subAgg.Hits.TotalHits.Value != 25365 {
		t.Fatalf("expected sub aggregation Hits.TotalHits.Value = %d; got: %d", 25365, subAgg.Hits.TotalHits.Value)
	}
	if subAgg.Hits.TotalHits.Relation != "eq" {
		t.Fatalf("expected sub aggregation Hits.TotalHits.Relation = %v; got: %v", "eq", subAgg.Hits.TotalHits.Relation)
	}
	if subAgg.Hits.MaxScore == nil {
		t.Fatalf("expected sub aggregation Hits.MaxScore != %v; got: %v", nil, *subAgg.Hits.MaxScore)
	}
	if *subAgg.Hits.MaxScore != float64(1.0) {
		t.Fatalf("expected sub aggregation Hits.MaxScore = %v; got: %v", float64(1.0), *subAgg.Hits.MaxScore)
	}

	subAgg, found = agg.Buckets[1].TopHits("top_tags_hits")
	if !found {
		t.Fatalf("expected sub aggregation to be found; got: %v", found)
	}
	if subAgg == nil {
		t.Fatalf("expected sub aggregation != nil; got: %v", subAgg)
	}
	if subAgg.Hits == nil {
		t.Fatalf("expected sub aggregation Hits != nil; got: %v", subAgg.Hits)
	}
	if subAgg.Hits.TotalHits == nil {
		t.Fatalf("expected sub aggregation Hits.TotalHits != nil; got: %v", subAgg.Hits.TotalHits)
	}
	if subAgg.Hits.TotalHits.Value != 18342 {
		t.Fatalf("expected sub aggregation Hits.TotalHits.Value = %d; got: %d", 18342, subAgg.Hits.TotalHits.Value)
	}
	if subAgg.Hits.TotalHits.Relation != "eq" {
		t.Fatalf("expected sub aggregation Hits.TotalHits.Relation = %v; got: %v", "eq", subAgg.Hits.TotalHits.Relation)
	}
	if subAgg.Hits.MaxScore == nil {
		t.Fatalf("expected sub aggregation Hits.MaxScore != %v; got: %v", nil, *subAgg.Hits.MaxScore)
	}
	if *subAgg.Hits.MaxScore != float64(1.0) {
		t.Fatalf("expected sub aggregation Hits.MaxScore = %v; got: %v", float64(1.0), *subAgg.Hits.MaxScore)
	}

	subAgg, found = agg.Buckets[2].TopHits("top_tags_hits")
	if !found {
		t.Fatalf("expected sub aggregation to be found; got: %v", found)
	}
	if subAgg == nil {
		t.Fatalf("expected sub aggregation != nil; got: %v", subAgg)
	}
	if subAgg.Hits == nil {
		t.Fatalf("expected sub aggregation Hits != nil; got: %v", subAgg.Hits)
	}
	if subAgg.Hits.TotalHits == nil {
		t.Fatalf("expected sub aggregation Hits.TotalHits != nil; got: %v", subAgg.Hits.TotalHits)
	}
	if subAgg.Hits.TotalHits.Value != 18119 {
		t.Fatalf("expected sub aggregation Hits.TotalHits.Value = %d; got: %d", 18119, subAgg.Hits.TotalHits.Value)
	}
	if subAgg.Hits.TotalHits.Relation != "eq" {
		t.Fatalf("expected sub aggregation Hits.TotalHits.Relation = %v; got: %v", "eq", subAgg.Hits.TotalHits.Relation)
	}
	if subAgg.Hits.MaxScore == nil {
		t.Fatalf("expected sub aggregation Hits.MaxScore != %v; got: %v", nil, *subAgg.Hits.MaxScore)
	}
	if *subAgg.Hits.MaxScore != float64(1.0) {
		t.Fatalf("expected sub aggregation Hits.MaxScore = %v; got: %v", float64(1.0), *subAgg.Hits.MaxScore)
	}
}

func TestAggsBucketGlobal(t *testing.T) {
	s := `{
	"all_products" : {
    "doc_count" : 100,
		"avg_price" : {
			"value" : 56.3
		}
	}
}`

	aggs := new(Aggregations)
	err := json.Unmarshal([]byte(s), &aggs)
	if err != nil {
		t.Fatalf("expected no error decoding; got: %v", err)
	}

	agg, found := aggs.Global("all_products")
	if !found {
		t.Fatalf("expected aggregation to be found; got: %v", found)
	}
	if agg == nil {
		t.Fatalf("expected aggregation != nil; got: %v", agg)
	}
	if agg.DocCount != 100 {
		t.Fatalf("expected aggregation DocCount = %d; got: %d", 100, agg.DocCount)
	}

	// Sub-aggregation
	subAgg, found := agg.Avg("avg_price")
	if !found {
		t.Fatalf("expected sub-aggregation to be found; got: %v", found)
	}
	if subAgg == nil {
		t.Fatalf("expected sub-aggregation != nil; got: %v", subAgg)
	}
	if subAgg.Value == nil {
		t.Fatalf("expected sub-aggregation value != nil; got: %v", subAgg.Value)
	}
	if *subAgg.Value != float64(56.3) {
		t.Fatalf("expected sub-aggregation value = %v; got: %v", float64(56.3), *subAgg.Value)
	}
}

func TestAggsBucketFilter(t *testing.T) {
	s := `{
	"in_stock_products" : {
	  "doc_count" : 100,
	  "avg_price" : { "value" : 56.3 }
	}
}`

	aggs := new(Aggregations)
	err := json.Unmarshal([]byte(s), &aggs)
	if err != nil {
		t.Fatalf("expected no error decoding; got: %v", err)
	}

	agg, found := aggs.Filter("in_stock_products")
	if !found {
		t.Fatalf("expected aggregation to be found; got: %v", found)
	}
	if agg == nil {
		t.Fatalf("expected aggregation != nil; got: %v", agg)
	}
	if agg.DocCount != 100 {
		t.Fatalf("expected aggregation DocCount = %d; got: %d", 100, agg.DocCount)
	}

	// Sub-aggregation
	subAgg, found := agg.Avg("avg_price")
	if !found {
		t.Fatalf("expected sub-aggregation to be found; got: %v", found)
	}
	if subAgg == nil {
		t.Fatalf("expected sub-aggregation != nil; got: %v", subAgg)
	}
	if subAgg.Value == nil {
		t.Fatalf("expected sub-aggregation value != nil; got: %v", subAgg.Value)
	}
	if *subAgg.Value != float64(56.3) {
		t.Fatalf("expected sub-aggregation value = %v; got: %v", float64(56.3), *subAgg.Value)
	}
}

func TestAggsBucketFiltersWithBuckets(t *testing.T) {
	s := `{
  "messages" : {
    "buckets" : [
    	{
        "doc_count" : 34,
        "monthly" : {
          "buckets" : []
        }
      },
      {
        "doc_count" : 439,
        "monthly" : {
          "buckets" : []
        }
      }
    ]
  }
}`

	aggs := new(Aggregations)
	err := json.Unmarshal([]byte(s), &aggs)
	if err != nil {
		t.Fatalf("expected no error decoding; got: %v", err)
	}

	agg, found := aggs.Filters("messages")
	if !found {
		t.Fatalf("expected aggregation to be found; got: %v", found)
	}
	if agg == nil {
		t.Fatalf("expected aggregation != nil; got: %v", agg)
	}
	if agg.Buckets == nil {
		t.Fatalf("expected aggregation buckets != %v; got: %v", nil, agg.Buckets)
	}
	if len(agg.Buckets) != 2 {
		t.Fatalf("expected %d buckets; got: %d", 2, len(agg.Buckets))
	}

	if agg.Buckets[0].DocCount != 34 {
		t.Fatalf("expected DocCount = %d; got: %d", 34, agg.Buckets[0].DocCount)
	}
	subAgg, found := agg.Buckets[0].Histogram("monthly")
	if !found {
		t.Fatalf("expected sub aggregation to be found; got: %v", found)
	}
	if subAgg == nil {
		t.Fatalf("expected sub aggregation != %v; got: %v", nil, subAgg)
	}

	if agg.Buckets[1].DocCount != 439 {
		t.Fatalf("expected DocCount = %d; got: %d", 439, agg.Buckets[1].DocCount)
	}
	subAgg, found = agg.Buckets[1].Histogram("monthly")
	if !found {
		t.Fatalf("expected sub aggregation to be found; got: %v", found)
	}
	if subAgg == nil {
		t.Fatalf("expected sub aggregation != %v; got: %v", nil, subAgg)
	}
}

func TestAggsBucketFiltersWithNamedBuckets(t *testing.T) {
	s := `{
  "messages" : {
    "buckets" : {
      "errors" : {
        "doc_count" : 34,
        "monthly" : {
          "buckets" : []
        }
      },
      "warnings" : {
        "doc_count" : 439,
        "monthly" : {
          "buckets" : []
        }
      }
    }
  }
}`

	aggs := new(Aggregations)
	err := json.Unmarshal([]byte(s), &aggs)
	if err != nil {
		t.Fatalf("expected no error decoding; got: %v", err)
	}

	agg, found := aggs.Filters("messages")
	if !found {
		t.Fatalf("expected aggregation to be found; got: %v", found)
	}
	if agg == nil {
		t.Fatalf("expected aggregation != nil; got: %v", agg)
	}
	if agg.NamedBuckets == nil {
		t.Fatalf("expected aggregation buckets != %v; got: %v", nil, agg.NamedBuckets)
	}
	if len(agg.NamedBuckets) != 2 {
		t.Fatalf("expected %d buckets; got: %d", 2, len(agg.NamedBuckets))
	}

	if agg.NamedBuckets["errors"].DocCount != 34 {
		t.Fatalf("expected DocCount = %d; got: %d", 34, agg.NamedBuckets["errors"].DocCount)
	}
	subAgg, found := agg.NamedBuckets["errors"].Histogram("monthly")
	if !found {
		t.Fatalf("expected sub aggregation to be found; got: %v", found)
	}
	if subAgg == nil {
		t.Fatalf("expected sub aggregation != %v; got: %v", nil, subAgg)
	}

	if agg.NamedBuckets["warnings"].DocCount != 439 {
		t.Fatalf("expected DocCount = %d; got: %d", 439, agg.NamedBuckets["warnings"].DocCount)
	}
	subAgg, found = agg.NamedBuckets["warnings"].Histogram("monthly")
	if !found {
		t.Fatalf("expected sub aggregation to be found; got: %v", found)
	}
	if subAgg == nil {
		t.Fatalf("expected sub aggregation != %v; got: %v", nil, subAgg)
	}
}

func TestAggsBucketAdjacencyMatrix(t *testing.T) {
	s := `{
	"interactions": {
		"buckets": [
			{
				"key": "grpA",
				"doc_count": 2,
				"monthly": {
					"buckets": []
				}
			},
			{
				"key": "grpA&grpB",
				"doc_count": 1,
				"monthly": {
					"buckets": []
				}
			}
		]
	}
}`

	aggs := new(Aggregations)
	err := json.Unmarshal([]byte(s), &aggs)
	if err != nil {
		t.Fatalf("expected no error decoding; got: %v", err)
	}

	agg, found := aggs.AdjacencyMatrix("interactions")
	if !found {
		t.Fatalf("expected aggregation to be found; got: %v", found)
	}
	if agg == nil {
		t.Fatalf("expected aggregation != nil; got: %v", agg)
	}
	if agg.Buckets == nil {
		t.Fatalf("expected aggregation buckets != %v; got: %v", nil, agg.Buckets)
	}
	if len(agg.Buckets) != 2 {
		t.Fatalf("expected %d buckets; got: %d", 2, len(agg.Buckets))
	}

	if agg.Buckets[0].DocCount != 2 {
		t.Fatalf("expected DocCount = %d; got: %d", 2, agg.Buckets[0].DocCount)
	}
	subAgg, found := agg.Buckets[0].Histogram("monthly")
	if !found {
		t.Fatalf("expected sub aggregation to be found; got: %v", found)
	}
	if subAgg == nil {
		t.Fatalf("expected sub aggregation != %v; got: %v", nil, subAgg)
	}

	if agg.Buckets[1].DocCount != 1 {
		t.Fatalf("expected DocCount = %d; got: %d", 1, agg.Buckets[1].DocCount)
	}
	subAgg, found = agg.Buckets[1].Histogram("monthly")
	if !found {
		t.Fatalf("expected sub aggregation to be found; got: %v", found)
	}
	if subAgg == nil {
		t.Fatalf("expected sub aggregation != %v; got: %v", nil, subAgg)
	}
}

func TestAggsBucketMissing(t *testing.T) {
	s := `{
	"products_without_a_price" : {
		"doc_count" : 10
	}
}`

	aggs := new(Aggregations)
	err := json.Unmarshal([]byte(s), &aggs)
	if err != nil {
		t.Fatalf("expected no error decoding; got: %v", err)
	}

	agg, found := aggs.Missing("products_without_a_price")
	if !found {
		t.Fatalf("expected aggregation to be found; got: %v", found)
	}
	if agg == nil {
		t.Fatalf("expected aggregation != nil; got: %v", agg)
	}
	if agg.DocCount != 10 {
		t.Fatalf("expected aggregation DocCount = %d; got: %d", 10, agg.DocCount)
	}
}

func TestAggsBucketNested(t *testing.T) {
	s := `{
	"resellers": {
		"min_price": {
			"value" : 350
		}
	}
}`

	aggs := new(Aggregations)
	err := json.Unmarshal([]byte(s), &aggs)
	if err != nil {
		t.Fatalf("expected no error decoding; got: %v", err)
	}

	agg, found := aggs.Nested("resellers")
	if !found {
		t.Fatalf("expected aggregation to be found; got: %v", found)
	}
	if agg == nil {
		t.Fatalf("expected aggregation != nil; got: %v", agg)
	}
	if agg.DocCount != 0 {
		t.Fatalf("expected aggregation DocCount = %d; got: %d", 0, agg.DocCount)
	}

	// Sub-aggregation
	subAgg, found := agg.Avg("min_price")
	if !found {
		t.Fatalf("expected sub-aggregation to be found; got: %v", found)
	}
	if subAgg == nil {
		t.Fatalf("expected sub-aggregation != nil; got: %v", subAgg)
	}
	if subAgg.Value == nil {
		t.Fatalf("expected sub-aggregation value != nil; got: %v", subAgg.Value)
	}
	if *subAgg.Value != float64(350) {
		t.Fatalf("expected sub-aggregation value = %v; got: %v", float64(350), *subAgg.Value)
	}
}

func TestAggsBucketReverseNested(t *testing.T) {
	s := `{
	"comment_to_issue": {
		"doc_count" : 10
	}
}`

	aggs := new(Aggregations)
	err := json.Unmarshal([]byte(s), &aggs)
	if err != nil {
		t.Fatalf("expected no error decoding; got: %v", err)
	}

	agg, found := aggs.ReverseNested("comment_to_issue")
	if !found {
		t.Fatalf("expected aggregation to be found; got: %v", found)
	}
	if agg == nil {
		t.Fatalf("expected aggregation != nil; got: %v", agg)
	}
	if agg.DocCount != 10 {
		t.Fatalf("expected aggregation DocCount = %d; got: %d", 10, agg.DocCount)
	}
}

func TestAggsBucketChildren(t *testing.T) {
	s := `{
	"to-answers": {
		"doc_count" : 10
	}
}`

	aggs := new(Aggregations)
	err := json.Unmarshal([]byte(s), &aggs)
	if err != nil {
		t.Fatalf("expected no error decoding; got: %v", err)
	}

	agg, found := aggs.Children("to-answers")
	if !found {
		t.Fatalf("expected aggregation to be found; got: %v", found)
	}
	if agg == nil {
		t.Fatalf("expected aggregation != nil; got: %v", agg)
	}
	if agg.DocCount != 10 {
		t.Fatalf("expected aggregation DocCount = %d; got: %d", 10, agg.DocCount)
	}
}

func TestAggsBucketTerms(t *testing.T) {
	s := `{
	"users" : {
	  "doc_count_error_upper_bound" : 1,
	  "sum_other_doc_count" : 2,
	  "buckets" : [ {
	    "key" : "olivere",
	    "doc_count" : 2
	  }, {
	    "key" : "sandrae",
	    "doc_count" : 1
	  } ]
	}
}`

	aggs := new(Aggregations)
	err := json.Unmarshal([]byte(s), &aggs)
	if err != nil {
		t.Fatalf("expected no error decoding; got: %v", err)
	}

	agg, found := aggs.Terms("users")
	if !found {
		t.Fatalf("expected aggregation to be found; got: %v", found)
	}
	if agg == nil {
		t.Fatalf("expected aggregation != nil; got: %v", agg)
	}
	if agg.Buckets == nil {
		t.Fatalf("expected aggregation buckets != nil; got: %v", agg.Buckets)
	}
	if len(agg.Buckets) != 2 {
		t.Errorf("expected %d bucket entries; got: %d", 2, len(agg.Buckets))
	}
	if agg.Buckets[0].Key != "olivere" {
		t.Errorf("expected key %q; got: %q", "olivere", agg.Buckets[0].Key)
	}
	if agg.Buckets[0].DocCount != 2 {
		t.Errorf("expected doc count %d; got: %d", 2, agg.Buckets[0].DocCount)
	}
	if agg.Buckets[1].Key != "sandrae" {
		t.Errorf("expected key %q; got: %q", "sandrae", agg.Buckets[1].Key)
	}
	if agg.Buckets[1].DocCount != 1 {
		t.Errorf("expected doc count %d; got: %d", 1, agg.Buckets[1].DocCount)
	}
}

func TestAggsBucketTermsWithNumericKeys(t *testing.T) {
	s := `{
	"users" : {
	  "doc_count_error_upper_bound" : 1,
	  "sum_other_doc_count" : 2,
	  "buckets" : [ {
	    "key" : 17,
	    "doc_count" : 2
	  }, {
	    "key" : 21,
	    "doc_count" : 1
	  } ]
	}
}`

	aggs := new(Aggregations)
	err := json.Unmarshal([]byte(s), &aggs)
	if err != nil {
		t.Fatalf("expected no error decoding; got: %v", err)
	}

	agg, found := aggs.Terms("users")
	if !found {
		t.Fatalf("expected aggregation to be found; got: %v", found)
	}
	if agg == nil {
		t.Fatalf("expected aggregation != nil; got: %v", agg)
	}
	if agg.Buckets == nil {
		t.Fatalf("expected aggregation buckets != nil; got: %v", agg.Buckets)
	}
	if len(agg.Buckets) != 2 {
		t.Errorf("expected %d bucket entries; got: %d", 2, len(agg.Buckets))
	}
	if agg.Buckets[0].Key != float64(17) {
		t.Errorf("expected key %v; got: %v", 17, agg.Buckets[0].Key)
	}
	if got, err := agg.Buckets[0].KeyNumber.Int64(); err != nil {
		t.Errorf("expected to convert key to int64; got: %v", err)
	} else if got != 17 {
		t.Errorf("expected key %v; got: %v", 17, agg.Buckets[0].Key)
	}
	if agg.Buckets[0].DocCount != 2 {
		t.Errorf("expected doc count %d; got: %d", 2, agg.Buckets[0].DocCount)
	}
	if agg.Buckets[1].Key != float64(21) {
		t.Errorf("expected key %v; got: %v", 21, agg.Buckets[1].Key)
	}
	if got, err := agg.Buckets[1].KeyNumber.Int64(); err != nil {
		t.Errorf("expected to convert key to int64; got: %v", err)
	} else if got != 21 {
		t.Errorf("expected key %v; got: %v", 21, agg.Buckets[1].Key)
	}
	if agg.Buckets[1].DocCount != 1 {
		t.Errorf("expected doc count %d; got: %d", 1, agg.Buckets[1].DocCount)
	}
}

func TestAggsBucketTermsWithBoolKeys(t *testing.T) {
	s := `{
	"users" : {
	  "doc_count_error_upper_bound" : 1,
	  "sum_other_doc_count" : 2,
	  "buckets" : [ {
	    "key" : true,
	    "doc_count" : 2
	  }, {
	    "key" : false,
	    "doc_count" : 1
	  } ]
	}
}`

	aggs := new(Aggregations)
	err := json.Unmarshal([]byte(s), &aggs)
	if err != nil {
		t.Fatalf("expected no error decoding; got: %v", err)
	}

	agg, found := aggs.Terms("users")
	if !found {
		t.Fatalf("expected aggregation to be found; got: %v", found)
	}
	if agg == nil {
		t.Fatalf("expected aggregation != nil; got: %v", agg)
	}
	if agg.Buckets == nil {
		t.Fatalf("expected aggregation buckets != nil; got: %v", agg.Buckets)
	}
	if len(agg.Buckets) != 2 {
		t.Errorf("expected %d bucket entries; got: %d", 2, len(agg.Buckets))
	}
	if agg.Buckets[0].Key != true {
		t.Errorf("expected key %v; got: %v", true, agg.Buckets[0].Key)
	}
	if agg.Buckets[0].DocCount != 2 {
		t.Errorf("expected doc count %d; got: %d", 2, agg.Buckets[0].DocCount)
	}
	if agg.Buckets[1].Key != false {
		t.Errorf("expected key %v; got: %v", false, agg.Buckets[1].Key)
	}
	if agg.Buckets[1].DocCount != 1 {
		t.Errorf("expected doc count %d; got: %d", 1, agg.Buckets[1].DocCount)
	}
}

func TestAggsBucketSignificantTerms(t *testing.T) {
	s := `{
	"significantCrimeTypes" : {
    "doc_count": 47347,
    "buckets" : [
      {
        "key": "Bicycle theft",
        "doc_count": 3640,
        "score": 0.371235374214817,
        "bg_count": 66799
      }
    ]
	}
}`

	aggs := new(Aggregations)
	err := json.Unmarshal([]byte(s), &aggs)
	if err != nil {
		t.Fatalf("expected no error decoding; got: %v", err)
	}

	agg, found := aggs.SignificantTerms("significantCrimeTypes")
	if !found {
		t.Fatalf("expected aggregation to be found; got: %v", found)
	}
	if agg == nil {
		t.Fatalf("expected aggregation != nil; got: %v", agg)
	}
	if agg.DocCount != 47347 {
		t.Fatalf("expected aggregation DocCount != %d; got: %d", 47347, agg.DocCount)
	}
	if agg.Buckets == nil {
		t.Fatalf("expected aggregation buckets != nil; got: %v", agg.Buckets)
	}
	if len(agg.Buckets) != 1 {
		t.Errorf("expected %d bucket entries; got: %d", 1, len(agg.Buckets))
	}
	if agg.Buckets[0].Key != "Bicycle theft" {
		t.Errorf("expected key = %q; got: %q", "Bicycle theft", agg.Buckets[0].Key)
	}
	if agg.Buckets[0].DocCount != 3640 {
		t.Errorf("expected doc count = %d; got: %d", 3640, agg.Buckets[0].DocCount)
	}
	if agg.Buckets[0].Score != float64(0.371235374214817) {
		t.Errorf("expected score = %v; got: %v", float64(0.371235374214817), agg.Buckets[0].Score)
	}
	if agg.Buckets[0].BgCount != 66799 {
		t.Errorf("expected BgCount = %d; got: %d", 66799, agg.Buckets[0].BgCount)
	}
}

func TestAggsBucketSampler(t *testing.T) {
	s := `{
	"sample" : {
    "doc_count": 1000,
    "keywords": {
    	"doc_count": 1000,
	    "buckets" : [
	      {
	        "key": "bend",
	        "doc_count": 58,
	        "score": 37.982536582524276,
	        "bg_count": 103
	      }
	    ]
    }
	}
}`

	aggs := new(Aggregations)
	err := json.Unmarshal([]byte(s), &aggs)
	if err != nil {
		t.Fatalf("expected no error decoding; got: %v", err)
	}

	agg, found := aggs.Sampler("sample")
	if !found {
		t.Fatalf("expected aggregation to be found; got: %v", found)
	}
	if agg == nil {
		t.Fatalf("expected aggregation != nil; got: %v", agg)
	}
	if agg.DocCount != 1000 {
		t.Fatalf("expected aggregation DocCount != %d; got: %d", 1000, agg.DocCount)
	}
	sub, found := agg.Aggregations["keywords"]
	if !found {
		t.Fatalf("expected sub aggregation %q", "keywords")
	}
	if sub == nil {
		t.Fatalf("expected sub aggregation %q; got: %v", "keywords", sub)
	}
}

func TestAggsBucketDiversifiedSampler(t *testing.T) {
	s := `{
	"diversified_sampler" : {
    "doc_count": 1000,
    "keywords": {
    	"doc_count": 1000,
	    "buckets" : [
	      {
	        "key": "bend",
	        "doc_count": 58,
	        "score": 37.982536582524276,
	        "bg_count": 103
	      }
	    ]
    }
	}
}`

	aggs := new(Aggregations)
	err := json.Unmarshal([]byte(s), &aggs)
	if err != nil {
		t.Fatalf("expected no error decoding; got: %v", err)
	}

	agg, found := aggs.DiversifiedSampler("diversified_sampler")
	if !found {
		t.Fatalf("expected aggregation to be found; got: %v", found)
	}
	if agg == nil {
		t.Fatalf("expected aggregation != nil; got: %v", agg)
	}
	if agg.DocCount != 1000 {
		t.Fatalf("expected aggregation DocCount != %d; got: %d", 1000, agg.DocCount)
	}
	sub, found := agg.Aggregations["keywords"]
	if !found {
		t.Fatalf("expected sub aggregation %q", "keywords")
	}
	if sub == nil {
		t.Fatalf("expected sub aggregation %q; got: %v", "keywords", sub)
	}
}

func TestAggsBucketRange(t *testing.T) {
	s := `{
	"price_ranges" : {
		"buckets": [
			{
				"to": 50,
				"doc_count": 2
			},
			{
				"from": 50,
				"to": 100,
				"doc_count": 4
			},
			{
				"from": 100,
				"doc_count": 4
			}
		]
	}
}`

	aggs := new(Aggregations)
	err := json.Unmarshal([]byte(s), &aggs)
	if err != nil {
		t.Fatalf("expected no error decoding; got: %v", err)
	}

	agg, found := aggs.Range("price_ranges")
	if !found {
		t.Fatalf("expected aggregation to be found; got: %v", found)
	}
	if agg == nil {
		t.Fatalf("expected aggregation != nil; got: %v", agg)
	}
	if agg.Buckets == nil {
		t.Fatalf("expected aggregation buckets != nil; got: %v", agg.Buckets)
	}
	if len(agg.Buckets) != 3 {
		t.Errorf("expected %d bucket entries; got: %d", 3, len(agg.Buckets))
	}
	if agg.Buckets[0].From != nil {
		t.Errorf("expected From = %v; got: %v", nil, agg.Buckets[0].From)
	}
	if agg.Buckets[0].To == nil {
		t.Errorf("expected To != %v; got: %v", nil, agg.Buckets[0].To)
	}
	if *agg.Buckets[0].To != float64(50) {
		t.Errorf("expected To = %v; got: %v", float64(50), *agg.Buckets[0].To)
	}
	if agg.Buckets[0].DocCount != 2 {
		t.Errorf("expected DocCount = %d; got: %d", 2, agg.Buckets[0].DocCount)
	}
	if agg.Buckets[1].From == nil {
		t.Errorf("expected From != %v; got: %v", nil, agg.Buckets[1].From)
	}
	if *agg.Buckets[1].From != float64(50) {
		t.Errorf("expected From = %v; got: %v", float64(50), *agg.Buckets[1].From)
	}
	if agg.Buckets[1].To == nil {
		t.Errorf("expected To != %v; got: %v", nil, agg.Buckets[1].To)
	}
	if *agg.Buckets[1].To != float64(100) {
		t.Errorf("expected To = %v; got: %v", float64(100), *agg.Buckets[1].To)
	}
	if agg.Buckets[1].DocCount != 4 {
		t.Errorf("expected DocCount = %d; got: %d", 4, agg.Buckets[1].DocCount)
	}
	if agg.Buckets[2].From == nil {
		t.Errorf("expected From != %v; got: %v", nil, agg.Buckets[2].From)
	}
	if *agg.Buckets[2].From != float64(100) {
		t.Errorf("expected From = %v; got: %v", float64(100), *agg.Buckets[2].From)
	}
	if agg.Buckets[2].To != nil {
		t.Errorf("expected To = %v; got: %v", nil, agg.Buckets[2].To)
	}
	if agg.Buckets[2].DocCount != 4 {
		t.Errorf("expected DocCount = %d; got: %d", 4, agg.Buckets[2].DocCount)
	}
}

func TestAggsBucketDateRange(t *testing.T) {
	s := `{
	"range": {
		"buckets": [
			{
				"to": 1.3437792E+12,
				"to_as_string": "08-2012",
				"doc_count": 7
			},
			{
				"from": 1.3437792E+12,
				"from_as_string": "08-2012",
				"doc_count": 2
			}
		]
	}
}`

	aggs := new(Aggregations)
	err := json.Unmarshal([]byte(s), &aggs)
	if err != nil {
		t.Fatalf("expected no error decoding; got: %v", err)
	}

	agg, found := aggs.DateRange("range")
	if !found {
		t.Fatalf("expected aggregation to be found; got: %v", found)
	}
	if agg == nil {
		t.Fatalf("expected aggregation != nil; got: %v", agg)
	}
	if agg.Buckets == nil {
		t.Fatalf("expected aggregation buckets != nil; got: %v", agg.Buckets)
	}
	if len(agg.Buckets) != 2 {
		t.Errorf("expected %d bucket entries; got: %d", 2, len(agg.Buckets))
	}
	if agg.Buckets[0].From != nil {
		t.Errorf("expected From = %v; got: %v", nil, agg.Buckets[0].From)
	}
	if agg.Buckets[0].To == nil {
		t.Errorf("expected To != %v; got: %v", nil, agg.Buckets[0].To)
	}
	if *agg.Buckets[0].To != float64(1.3437792E+12) {
		t.Errorf("expected To = %v; got: %v", float64(1.3437792E+12), *agg.Buckets[0].To)
	}
	if agg.Buckets[0].ToAsString != "08-2012" {
		t.Errorf("expected ToAsString = %q; got: %q", "08-2012", agg.Buckets[0].ToAsString)
	}
	if agg.Buckets[0].DocCount != 7 {
		t.Errorf("expected DocCount = %d; got: %d", 7, agg.Buckets[0].DocCount)
	}
	if agg.Buckets[1].From == nil {
		t.Errorf("expected From != %v; got: %v", nil, agg.Buckets[1].From)
	}
	if *agg.Buckets[1].From != float64(1.3437792E+12) {
		t.Errorf("expected From = %v; got: %v", float64(1.3437792E+12), *agg.Buckets[1].From)
	}
	if agg.Buckets[1].FromAsString != "08-2012" {
		t.Errorf("expected FromAsString = %q; got: %q", "08-2012", agg.Buckets[1].FromAsString)
	}
	if agg.Buckets[1].To != nil {
		t.Errorf("expected To = %v; got: %v", nil, agg.Buckets[1].To)
	}
	if agg.Buckets[1].DocCount != 2 {
		t.Errorf("expected DocCount = %d; got: %d", 2, agg.Buckets[1].DocCount)
	}
}

func TestAggsBucketIPRange(t *testing.T) {
	s := `{
	"ip_ranges": {
		"buckets" : [
			{
				"to": 167772165,
				"to_as_string": "10.0.0.5",
				"doc_count": 4
			},
			{
				"from": 167772165,
				"from_as_string": "10.0.0.5",
				"doc_count": 6
			}
		]
	}
}`

	aggs := new(Aggregations)
	err := json.Unmarshal([]byte(s), &aggs)
	if err != nil {
		t.Fatalf("expected no error decoding; got: %v", err)
	}

	agg, found := aggs.IPRange("ip_ranges")
	if !found {
		t.Fatalf("expected aggregation to be found; got: %v", found)
	}
	if agg == nil {
		t.Fatalf("expected aggregation != nil; got: %v", agg)
	}
	if agg.Buckets == nil {
		t.Fatalf("expected aggregation buckets != nil; got: %v", agg.Buckets)
	}
	if len(agg.Buckets) != 2 {
		t.Errorf("expected %d bucket entries; got: %d", 2, len(agg.Buckets))
	}
	if agg.Buckets[0].From != nil {
		t.Errorf("expected From = %v; got: %v", nil, agg.Buckets[0].From)
	}
	if agg.Buckets[0].To == nil {
		t.Errorf("expected To != %v; got: %v", nil, agg.Buckets[0].To)
	}
	if *agg.Buckets[0].To != float64(167772165) {
		t.Errorf("expected To = %v; got: %v", float64(167772165), *agg.Buckets[0].To)
	}
	if agg.Buckets[0].ToAsString != "10.0.0.5" {
		t.Errorf("expected ToAsString = %q; got: %q", "10.0.0.5", agg.Buckets[0].ToAsString)
	}
	if agg.Buckets[0].DocCount != 4 {
		t.Errorf("expected DocCount = %d; got: %d", 4, agg.Buckets[0].DocCount)
	}
	if agg.Buckets[1].From == nil {
		t.Errorf("expected From != %v; got: %v", nil, agg.Buckets[1].From)
	}
	if *agg.Buckets[1].From != float64(167772165) {
		t.Errorf("expected From = %v; got: %v", float64(167772165), *agg.Buckets[1].From)
	}
	if agg.Buckets[1].FromAsString != "10.0.0.5" {
		t.Errorf("expected FromAsString = %q; got: %q", "10.0.0.5", agg.Buckets[1].FromAsString)
	}
	if agg.Buckets[1].To != nil {
		t.Errorf("expected To = %v; got: %v", nil, agg.Buckets[1].To)
	}
	if agg.Buckets[1].DocCount != 6 {
		t.Errorf("expected DocCount = %d; got: %d", 6, agg.Buckets[1].DocCount)
	}
}

func TestAggsBucketHistogram(t *testing.T) {
	s := `{
	"prices" : {
		"buckets": [
			{
				"key": 0,
				"doc_count": 2
			},
			{
				"key": 50,
				"doc_count": 4
			},
			{
				"key": 150,
				"doc_count": 3
			}
		]
	}
}`

	aggs := new(Aggregations)
	err := json.Unmarshal([]byte(s), &aggs)
	if err != nil {
		t.Fatalf("expected no error decoding; got: %v", err)
	}

	agg, found := aggs.Histogram("prices")
	if !found {
		t.Fatalf("expected aggregation to be found; got: %v", found)
	}
	if agg == nil {
		t.Fatalf("expected aggregation != nil; got: %v", agg)
	}
	if agg.Buckets == nil {
		t.Fatalf("expected aggregation buckets != nil; got: %v", agg.Buckets)
	}
	if len(agg.Buckets) != 3 {
		t.Errorf("expected %d buckets; got: %d", 3, len(agg.Buckets))
	}
	if agg.Buckets[0].Key != 0 {
		t.Errorf("expected key = %v; got: %v", 0, agg.Buckets[0].Key)
	}
	if agg.Buckets[0].KeyAsString != nil {
		t.Fatalf("expected key_as_string = %v; got: %q", nil, *agg.Buckets[0].KeyAsString)
	}
	if agg.Buckets[0].DocCount != 2 {
		t.Errorf("expected doc count = %d; got: %d", 2, agg.Buckets[0].DocCount)
	}
	if agg.Buckets[1].Key != 50 {
		t.Errorf("expected key = %v; got: %v", 50, agg.Buckets[1].Key)
	}
	if agg.Buckets[1].KeyAsString != nil {
		t.Fatalf("expected key_as_string = %v; got: %q", nil, *agg.Buckets[1].KeyAsString)
	}
	if agg.Buckets[1].DocCount != 4 {
		t.Errorf("expected doc count = %d; got: %d", 4, agg.Buckets[1].DocCount)
	}
	if agg.Buckets[2].Key != 150 {
		t.Errorf("expected key = %v; got: %v", 150, agg.Buckets[2].Key)
	}
	if agg.Buckets[2].KeyAsString != nil {
		t.Fatalf("expected key_as_string = %v; got: %q", nil, *agg.Buckets[2].KeyAsString)
	}
	if agg.Buckets[2].DocCount != 3 {
		t.Errorf("expected doc count = %d; got: %d", 3, agg.Buckets[2].DocCount)
	}
}

func TestAggsBucketDateHistogram(t *testing.T) {
	s := `{
	"articles_over_time": {
	  "buckets": [
	      {
	          "key_as_string": "2013-02-02",
	          "key": 1328140800000,
	          "doc_count": 1
	      },
	      {
	          "key_as_string": "2013-03-02",
	          "key": 1330646400000,
	          "doc_count": 2
	      }
	  ]
	}
}`

	aggs := new(Aggregations)
	err := json.Unmarshal([]byte(s), &aggs)
	if err != nil {
		t.Fatalf("expected no error decoding; got: %v", err)
	}

	agg, found := aggs.DateHistogram("articles_over_time")
	if !found {
		t.Fatalf("expected aggregation to be found; got: %v", found)
	}
	if agg == nil {
		t.Fatalf("expected aggregation != nil; got: %v", agg)
	}
	if agg.Buckets == nil {
		t.Fatalf("expected aggregation buckets != nil; got: %v", agg.Buckets)
	}
	if len(agg.Buckets) != 2 {
		t.Errorf("expected %d bucket entries; got: %d", 2, len(agg.Buckets))
	}
	if agg.Buckets[0].Key != 1328140800000 {
		t.Errorf("expected key %v; got: %v", 1328140800000, agg.Buckets[0].Key)
	}
	if agg.Buckets[0].KeyAsString == nil {
		t.Fatalf("expected key_as_string != nil; got: %v", agg.Buckets[0].KeyAsString)
	}
	if *agg.Buckets[0].KeyAsString != "2013-02-02" {
		t.Errorf("expected key_as_string %q; got: %q", "2013-02-02", *agg.Buckets[0].KeyAsString)
	}
	if agg.Buckets[0].DocCount != 1 {
		t.Errorf("expected doc count %d; got: %d", 1, agg.Buckets[0].DocCount)
	}
	if agg.Buckets[1].Key != 1330646400000 {
		t.Errorf("expected key %v; got: %v", 1330646400000, agg.Buckets[1].Key)
	}
	if agg.Buckets[1].KeyAsString == nil {
		t.Fatalf("expected key_as_string != nil; got: %v", agg.Buckets[1].KeyAsString)
	}
	if *agg.Buckets[1].KeyAsString != "2013-03-02" {
		t.Errorf("expected key_as_string %q; got: %q", "2013-03-02", *agg.Buckets[1].KeyAsString)
	}
	if agg.Buckets[1].DocCount != 2 {
		t.Errorf("expected doc count %d; got: %d", 2, agg.Buckets[1].DocCount)
	}
}

func TestAggsMetricsGeoBounds(t *testing.T) {
	s := `{
  "viewport": {
    "bounds": {
      "top_left": {
        "lat": 80.45,
        "lon": -160.22
      },
      "bottom_right": {
        "lat": 40.65,
        "lon": 42.57
      }
    }
  }
}`

	aggs := new(Aggregations)
	err := json.Unmarshal([]byte(s), &aggs)
	if err != nil {
		t.Fatalf("expected no error decoding; got: %v", err)
	}

	agg, found := aggs.GeoBounds("viewport")
	if !found {
		t.Fatalf("expected aggregation to be found; got: %v", found)
	}
	if agg == nil {
		t.Fatalf("expected aggregation != nil; got: %v", agg)
	}
	if agg.Bounds.TopLeft.Latitude != float64(80.45) {
		t.Fatalf("expected Bounds.TopLeft.Latitude != %v; got: %v", float64(80.45), agg.Bounds.TopLeft.Latitude)
	}
	if agg.Bounds.TopLeft.Longitude != float64(-160.22) {
		t.Fatalf("expected Bounds.TopLeft.Longitude != %v; got: %v", float64(-160.22), agg.Bounds.TopLeft.Longitude)
	}
	if agg.Bounds.BottomRight.Latitude != float64(40.65) {
		t.Fatalf("expected Bounds.BottomRight.Latitude != %v; got: %v", float64(40.65), agg.Bounds.BottomRight.Latitude)
	}
	if agg.Bounds.BottomRight.Longitude != float64(42.57) {
		t.Fatalf("expected Bounds.BottomRight.Longitude != %v; got: %v", float64(42.57), agg.Bounds.BottomRight.Longitude)
	}
}

func TestAggsBucketGeoHash(t *testing.T) {
	s := `{
	"myLarge-GrainGeoHashGrid": {
		"buckets": [
			{
				"key": "svz",
				"doc_count": 10964
			},
			{
				"key": "sv8",
				"doc_count": 3198
			}
		]
	}
}`

	aggs := new(Aggregations)
	err := json.Unmarshal([]byte(s), &aggs)
	if err != nil {
		t.Fatalf("expected no error decoding; got: %v", err)
	}

	agg, found := aggs.GeoHash("myLarge-GrainGeoHashGrid")
	if !found {
		t.Fatalf("expected aggregation to be found; got: %v", found)
	}
	if agg == nil {
		t.Fatalf("expected aggregation != nil; got: %v", agg)
	}
	if agg.Buckets == nil {
		t.Fatalf("expected aggregation buckets != nil; got: %v", agg.Buckets)
	}
	if len(agg.Buckets) != 2 {
		t.Errorf("expected %d bucket entries; got: %d", 2, len(agg.Buckets))
	}
	if agg.Buckets[0].Key != "svz" {
		t.Errorf("expected key %q; got: %q", "svz", agg.Buckets[0].Key)
	}
	if agg.Buckets[0].DocCount != 10964 {
		t.Errorf("expected doc count %d; got: %d", 10964, agg.Buckets[0].DocCount)
	}
	if agg.Buckets[1].Key != "sv8" {
		t.Errorf("expected key %q; got: %q", "sv8", agg.Buckets[1].Key)
	}
	if agg.Buckets[1].DocCount != 3198 {
		t.Errorf("expected doc count %d; got: %d", 3198, agg.Buckets[1].DocCount)
	}
}

func TestAggsMetricsGeoCentroid(t *testing.T) {
	s := `{
  "centroid": {
    "location": {
		"lat": 80.45,
		"lon": -160.22
    },
	"count": 6
  }
}`

	aggs := new(Aggregations)
	err := json.Unmarshal([]byte(s), &aggs)
	if err != nil {
		t.Fatalf("expected no error decoding; got: %v", err)
	}

	agg, found := aggs.GeoCentroid("centroid")
	if !found {
		t.Fatalf("expected aggregation to be found; got: %v", found)
	}
	if agg == nil {
		t.Fatalf("expected aggregation != nil; got: %v", agg)
	}
	if agg.Location.Latitude != float64(80.45) {
		t.Fatalf("expected Location.Latitude != %v; got: %v", float64(80.45), agg.Location.Latitude)
	}
	if agg.Location.Longitude != float64(-160.22) {
		t.Fatalf("expected Location.Longitude != %v; got: %v", float64(-160.22), agg.Location.Longitude)
	}
	if agg.Count != int(6) {
		t.Fatalf("expected Count != %v; got: %v", int(6), agg.Count)
	}
}

func TestAggsBucketGeoDistance(t *testing.T) {
	s := `{
	"rings" : {
		"buckets": [
			{
				"unit": "km",
				"to": 100.0,
				"doc_count": 3
			},
			{
				"unit": "km",
				"from": 100.0,
				"to": 300.0,
				"doc_count": 1
			},
			{
				"unit": "km",
				"from": 300.0,
				"doc_count": 7
			}
		]
	}
}`

	aggs := new(Aggregations)
	err := json.Unmarshal([]byte(s), &aggs)
	if err != nil {
		t.Fatalf("expected no error decoding; got: %v", err)
	}

	agg, found := aggs.GeoDistance("rings")
	if !found {
		t.Fatalf("expected aggregation to be found; got: %v", found)
	}
	if agg == nil {
		t.Fatalf("expected aggregation != nil; got: %v", agg)
	}
	if agg.Buckets == nil {
		t.Fatalf("expected aggregation buckets != nil; got: %v", agg.Buckets)
	}
	if len(agg.Buckets) != 3 {
		t.Errorf("expected %d bucket entries; got: %d", 3, len(agg.Buckets))
	}
	if agg.Buckets[0].From != nil {
		t.Errorf("expected From = %v; got: %v", nil, agg.Buckets[0].From)
	}
	if agg.Buckets[0].To == nil {
		t.Errorf("expected To != %v; got: %v", nil, agg.Buckets[0].To)
	}
	if *agg.Buckets[0].To != float64(100.0) {
		t.Errorf("expected To = %v; got: %v", float64(100.0), *agg.Buckets[0].To)
	}
	if agg.Buckets[0].DocCount != 3 {
		t.Errorf("expected DocCount = %d; got: %d", 4, agg.Buckets[0].DocCount)
	}

	if agg.Buckets[1].From == nil {
		t.Errorf("expected From != %v; got: %v", nil, agg.Buckets[1].From)
	}
	if *agg.Buckets[1].From != float64(100.0) {
		t.Errorf("expected From = %v; got: %v", float64(100.0), *agg.Buckets[1].From)
	}
	if agg.Buckets[1].To == nil {
		t.Errorf("expected To != %v; got: %v", nil, agg.Buckets[1].To)
	}
	if *agg.Buckets[1].To != float64(300.0) {
		t.Errorf("expected From = %v; got: %v", float64(300.0), *agg.Buckets[1].To)
	}
	if agg.Buckets[1].DocCount != 1 {
		t.Errorf("expected DocCount = %d; got: %d", 1, agg.Buckets[1].DocCount)
	}

	if agg.Buckets[2].From == nil {
		t.Errorf("expected From != %v; got: %v", nil, agg.Buckets[2].From)
	}
	if *agg.Buckets[2].From != float64(300.0) {
		t.Errorf("expected From = %v; got: %v", float64(300.0), *agg.Buckets[2].From)
	}
	if agg.Buckets[2].To != nil {
		t.Errorf("expected To = %v; got: %v", nil, agg.Buckets[2].To)
	}
	if agg.Buckets[2].DocCount != 7 {
		t.Errorf("expected DocCount = %d; got: %d", 7, agg.Buckets[2].DocCount)
	}
}

func TestAggsSubAggregates(t *testing.T) {
	rs := `{
	"users" : {
	  "doc_count_error_upper_bound" : 1,
	  "sum_other_doc_count" : 2,
	  "buckets" : [ {
	    "key" : "olivere",
	    "doc_count" : 2,
	    "ts" : {
	      "buckets" : [ {
	        "key_as_string" : "2012-01-01T00:00:00.000Z",
	        "key" : 1325376000000,
	        "doc_count" : 2
	      } ]
	    }
	  }, {
	    "key" : "sandrae",
	    "doc_count" : 1,
	    "ts" : {
	      "buckets" : [ {
	        "key_as_string" : "2011-01-01T00:00:00.000Z",
	        "key" : 1293840000000,
	        "doc_count" : 1
	      } ]
	    }
	  } ]
	}
}`

	aggs := new(Aggregations)
	err := json.Unmarshal([]byte(rs), &aggs)
	if err != nil {
		t.Fatalf("expected no error decoding; got: %v", err)
	}

	// Access top-level aggregation
	users, found := aggs.Terms("users")
	if !found {
		t.Fatalf("expected users aggregation to be found; got: %v", found)
	}
	if users == nil {
		t.Fatalf("expected users aggregation; got: %v", users)
	}
	if users.Buckets == nil {
		t.Fatalf("expected users buckets; got: %v", users.Buckets)
	}
	if len(users.Buckets) != 2 {
		t.Errorf("expected %d bucket entries; got: %d", 2, len(users.Buckets))
	}
	if users.Buckets[0].Key != "olivere" {
		t.Errorf("expected key %q; got: %q", "olivere", users.Buckets[0].Key)
	}
	if users.Buckets[0].DocCount != 2 {
		t.Errorf("expected doc count %d; got: %d", 2, users.Buckets[0].DocCount)
	}
	if users.Buckets[1].Key != "sandrae" {
		t.Errorf("expected key %q; got: %q", "sandrae", users.Buckets[1].Key)
	}
	if users.Buckets[1].DocCount != 1 {
		t.Errorf("expected doc count %d; got: %d", 1, users.Buckets[1].DocCount)
	}

	// Access sub-aggregation
	ts, found := users.Buckets[0].DateHistogram("ts")
	if !found {
		t.Fatalf("expected ts aggregation to be found; got: %v", found)
	}
	if ts == nil {
		t.Fatalf("expected ts aggregation; got: %v", ts)
	}
	if ts.Buckets == nil {
		t.Fatalf("expected ts buckets; got: %v", ts.Buckets)
	}
	if len(ts.Buckets) != 1 {
		t.Errorf("expected %d bucket entries; got: %d", 1, len(ts.Buckets))
	}
	if ts.Buckets[0].Key != 1325376000000 {
		t.Errorf("expected key %v; got: %v", 1325376000000, ts.Buckets[0].Key)
	}
	if ts.Buckets[0].KeyAsString == nil {
		t.Fatalf("expected key_as_string != %v; got: %v", nil, ts.Buckets[0].KeyAsString)
	}
	if *ts.Buckets[0].KeyAsString != "2012-01-01T00:00:00.000Z" {
		t.Errorf("expected key_as_string %q; got: %q", "2012-01-01T00:00:00.000Z", *ts.Buckets[0].KeyAsString)
	}
}

func TestAggsPipelineAvgBucket(t *testing.T) {
	s := `{
	"avg_monthly_sales" : {
	  "value" : 328.33333333333333
  }
}`

	aggs := new(Aggregations)
	err := json.Unmarshal([]byte(s), &aggs)
	if err != nil {
		t.Fatalf("expected no error decoding; got: %v", err)
	}

	agg, found := aggs.AvgBucket("avg_monthly_sales")
	if !found {
		t.Fatalf("expected aggregation to be found; got: %v", found)
	}
	if agg == nil {
		t.Fatalf("expected aggregation != nil; got: %v", agg)
	}
	if agg.Value == nil {
		t.Fatalf("expected aggregation value != nil; got: %v", agg.Value)
	}
	if *agg.Value != float64(328.33333333333333) {
		t.Fatalf("expected aggregation value = %v; got: %v", float64(328.33333333333333), *agg.Value)
	}
}

func TestAggsPipelineSumBucket(t *testing.T) {
	s := `{
	"sum_monthly_sales" : {
	  "value" : 985
  }
}`

	aggs := new(Aggregations)
	err := json.Unmarshal([]byte(s), &aggs)
	if err != nil {
		t.Fatalf("expected no error decoding; got: %v", err)
	}

	agg, found := aggs.SumBucket("sum_monthly_sales")
	if !found {
		t.Fatalf("expected aggregation to be found; got: %v", found)
	}
	if agg == nil {
		t.Fatalf("expected aggregation != nil; got: %v", agg)
	}
	if agg.Value == nil {
		t.Fatalf("expected aggregation value != nil; got: %v", agg.Value)
	}
	if *agg.Value != float64(985) {
		t.Fatalf("expected aggregation value = %v; got: %v", float64(985), *agg.Value)
	}
}

func TestAggsPipelineMaxBucket(t *testing.T) {
	s := `{
	"max_monthly_sales" : {
		"keys": ["2015/01/01 00:00:00"],
	  "value" : 550
  }
}`

	aggs := new(Aggregations)
	err := json.Unmarshal([]byte(s), &aggs)
	if err != nil {
		t.Fatalf("expected no error decoding; got: %v", err)
	}

	agg, found := aggs.MaxBucket("max_monthly_sales")
	if !found {
		t.Fatalf("expected aggregation to be found; got: %v", found)
	}
	if agg == nil {
		t.Fatalf("expected aggregation != nil; got: %v", agg)
	}
	if len(agg.Keys) != 1 {
		t.Fatalf("expected 1 key; got: %d", len(agg.Keys))
	}
	if got, want := agg.Keys[0], "2015/01/01 00:00:00"; got != want {
		t.Fatalf("expected key %q; got: %v (%T)", want, got, got)
	}
	if agg.Value == nil {
		t.Fatalf("expected aggregation value != nil; got: %v", agg.Value)
	}
	if *agg.Value != float64(550) {
		t.Fatalf("expected aggregation value = %v; got: %v", float64(550), *agg.Value)
	}
}

func TestAggsPipelineMinBucket(t *testing.T) {
	s := `{
	"min_monthly_sales" : {
		"keys": ["2015/02/01 00:00:00"],
	  "value" : 60
  }
}`

	aggs := new(Aggregations)
	err := json.Unmarshal([]byte(s), &aggs)
	if err != nil {
		t.Fatalf("expected no error decoding; got: %v", err)
	}

	agg, found := aggs.MinBucket("min_monthly_sales")
	if !found {
		t.Fatalf("expected aggregation to be found; got: %v", found)
	}
	if agg == nil {
		t.Fatalf("expected aggregation != nil; got: %v", agg)
	}
	if len(agg.Keys) != 1 {
		t.Fatalf("expected 1 key; got: %d", len(agg.Keys))
	}
	if got, want := agg.Keys[0], "2015/02/01 00:00:00"; got != want {
		t.Fatalf("expected key %q; got: %v (%T)", want, got, got)
	}
	if agg.Value == nil {
		t.Fatalf("expected aggregation value != nil; got: %v", agg.Value)
	}
	if *agg.Value != float64(60) {
		t.Fatalf("expected aggregation value = %v; got: %v", float64(60), *agg.Value)
	}
}

func TestAggsPipelineMovAvg(t *testing.T) {
	s := `{
	"the_movavg" : {
	  "value" : 12.0
  }
}`

	aggs := new(Aggregations)
	err := json.Unmarshal([]byte(s), &aggs)
	if err != nil {
		t.Fatalf("expected no error decoding; got: %v", err)
	}

	agg, found := aggs.MovAvg("the_movavg")
	if !found {
		t.Fatalf("expected aggregation to be found; got: %v", found)
	}
	if agg == nil {
		t.Fatalf("expected aggregation != nil; got: %v", agg)
	}
	if agg.Value == nil {
		t.Fatalf("expected aggregation value != nil; got: %v", agg.Value)
	}
	if *agg.Value != float64(12.0) {
		t.Fatalf("expected aggregation value = %v; got: %v", float64(12.0), *agg.Value)
	}
}

func TestAggsPipelineDerivative(t *testing.T) {
	s := `{
	"sales_deriv" : {
	  "value" : 315
  }
}`

	aggs := new(Aggregations)
	err := json.Unmarshal([]byte(s), &aggs)
	if err != nil {
		t.Fatalf("expected no error decoding; got: %v", err)
	}

	agg, found := aggs.Derivative("sales_deriv")
	if !found {
		t.Fatalf("expected aggregation to be found; got: %v", found)
	}
	if agg == nil {
		t.Fatalf("expected aggregation != nil; got: %v", agg)
	}
	if agg.Value == nil {
		t.Fatalf("expected aggregation value != nil; got: %v", agg.Value)
	}
	if *agg.Value != float64(315) {
		t.Fatalf("expected aggregation value = %v; got: %v", float64(315), *agg.Value)
	}
}

func TestAggsPipelinePercentilesBucket(t *testing.T) {
	s := `{
	"sales_percentiles": {
	  "values": {
        "25.0": 100,
        "50.0": 200,
        "75.0": 300
      }
    }
}`
	aggs := new(Aggregations)
	err := json.Unmarshal([]byte(s), &aggs)
	if err != nil {
		t.Fatalf("expected no error decoding; got: %v", err)
	}

	agg, found := aggs.PercentilesBucket("sales_percentiles")
	if !found {
		t.Fatalf("expected aggregation to be found; got: %v", found)
	}
	if agg == nil {
		t.Fatalf("expected aggregation != nil; got: %v", agg)
	}
	if len(agg.Values) != 3 {
		t.Fatalf("expected aggregation map with three entries; got: %v", agg.Values)
	}
}

func TestAggsPipelineStatsBucket(t *testing.T) {
	s := `{
	"stats_monthly_sales": {
	 "count": 3,
	 "min": 60.0,
	 "max": 550.0,
	 "avg": 328.3333333333333,
	 "sum": 985.0
  }
}`

	aggs := new(Aggregations)
	err := json.Unmarshal([]byte(s), &aggs)
	if err != nil {
		t.Fatalf("expected no error decoding; got: %v", err)
	}

	agg, found := aggs.StatsBucket("stats_monthly_sales")
	if !found {
		t.Fatalf("expected aggregation to be found; got: %v", found)
	}
	if agg == nil {
		t.Fatalf("expected aggregation != nil; got: %v", agg)
	}
	if agg.Count != 3 {
		t.Fatalf("expected aggregation count = %v; got: %v", 3, agg.Count)
	}
	if agg.Min == nil {
		t.Fatalf("expected aggregation min != nil; got: %v", agg.Min)
	}
	if *agg.Min != float64(60.0) {
		t.Fatalf("expected aggregation min = %v; got: %v", float64(60.0), *agg.Min)
	}
	if agg.Max == nil {
		t.Fatalf("expected aggregation max != nil; got: %v", agg.Max)
	}
	if *agg.Max != float64(550.0) {
		t.Fatalf("expected aggregation max = %v; got: %v", float64(550.0), *agg.Max)
	}
	if agg.Avg == nil {
		t.Fatalf("expected aggregation avg != nil; got: %v", agg.Avg)
	}
	if *agg.Avg != float64(328.3333333333333) {
		t.Fatalf("expected aggregation average = %v; got: %v", float64(328.3333333333333), *agg.Avg)
	}
	if agg.Sum == nil {
		t.Fatalf("expected aggregation sum != nil; got: %v", agg.Sum)
	}
	if *agg.Sum != float64(985.0) {
		t.Fatalf("expected aggregation sum = %v; got: %v", float64(985.0), *agg.Sum)
	}
}

func TestAggsPipelineCumulativeSum(t *testing.T) {
	s := `{
	"cumulative_sales" : {
	  "value" : 550
  }
}`

	aggs := new(Aggregations)
	err := json.Unmarshal([]byte(s), &aggs)
	if err != nil {
		t.Fatalf("expected no error decoding; got: %v", err)
	}

	agg, found := aggs.CumulativeSum("cumulative_sales")
	if !found {
		t.Fatalf("expected aggregation to be found; got: %v", found)
	}
	if agg == nil {
		t.Fatalf("expected aggregation != nil; got: %v", agg)
	}
	if agg.Value == nil {
		t.Fatalf("expected aggregation value != nil; got: %v", agg.Value)
	}
	if *agg.Value != float64(550) {
		t.Fatalf("expected aggregation value = %v; got: %v", float64(550), *agg.Value)
	}
}

func TestAggsPipelineBucketScript(t *testing.T) {
	s := `{
	"t-shirt-percentage" : {
	  "value" : 20
  }
}`

	aggs := new(Aggregations)
	err := json.Unmarshal([]byte(s), &aggs)
	if err != nil {
		t.Fatalf("expected no error decoding; got: %v", err)
	}

	agg, found := aggs.BucketScript("t-shirt-percentage")
	if !found {
		t.Fatalf("expected aggregation to be found; got: %v", found)
	}
	if agg == nil {
		t.Fatalf("expected aggregation != nil; got: %v", agg)
	}
	if agg.Value == nil {
		t.Fatalf("expected aggregation value != nil; got: %v", agg.Value)
	}
	if *agg.Value != float64(20) {
		t.Fatalf("expected aggregation value = %v; got: %v", float64(20), *agg.Value)
	}
}

func TestAggsPipelineSerialDiff(t *testing.T) {
	s := `{
	"the_diff" : {
	  "value" : -722.0
  }
}`

	aggs := new(Aggregations)
	err := json.Unmarshal([]byte(s), &aggs)
	if err != nil {
		t.Fatalf("expected no error decoding; got: %v", err)
	}

	agg, found := aggs.SerialDiff("the_diff")
	if !found {
		t.Fatalf("expected aggregation to be found; got: %v", found)
	}
	if agg == nil {
		t.Fatalf("expected aggregation != nil; got: %v", agg)
	}
	if agg.Value == nil {
		t.Fatalf("expected aggregation value != nil; got: %v", agg.Value)
	}
	if *agg.Value != float64(-722.0) {
		t.Fatalf("expected aggregation value = %v; got: %v", float64(20), *agg.Value)
	}
}

func TestAggsComposite(t *testing.T) {
	s := `{
	"the_composite" : {
		"buckets" : [
		  {
			"key" : {
			  "composite_users" : "olivere",
			  "composite_retweets" : 0.0,
			  "composite_created" : 1349856720000
			},
			"doc_count" : 1
		  },
		  {
			"key" : {
			  "composite_users" : "olivere",
			  "composite_retweets" : 108.0,
			  "composite_created" : 1355333880000
			},
			"doc_count" : 1
		  },
		  {
			"key" : {
			  "composite_users" : "sandrae",
			  "composite_retweets" : 12.0,
			  "composite_created" : 1321009080000
			},
			"doc_count" : 1
		  }
		]
	  }
	}`

	aggs := new(Aggregations)
	err := json.Unmarshal([]byte(s), &aggs)
	if err != nil {
		t.Fatalf("expected no error decoding; got: %v", err)
	}

	agg, found := aggs.Composite("the_composite")
	if !found {
		t.Fatalf("expected aggregation to be found; got: %v", found)
	}
	if agg == nil {
		t.Fatalf("expected aggregation != nil; got: %v", agg)
	}
	if want, have := 3, len(agg.Buckets); want != have {
		t.Fatalf("expected aggregation buckets length = %v; got: %v", want, have)
	}

	// 1st bucket
	bucket := agg.Buckets[0]
	if want, have := int64(1), bucket.DocCount; want != have {
		t.Fatalf("expected aggregation bucket doc count = %v; got: %v", want, have)
	}
	if want, have := 3, len(bucket.Key); want != have {
		t.Fatalf("expected aggregation bucket key length = %v; got: %v", want, have)
	}
	v, found := bucket.Key["composite_users"]
	if !found {
		t.Fatalf("expected to find bucket key %q", "composite_users")
	}
	s, ok := v.(string)
	if !ok {
		t.Fatalf("expected to have bucket key of type string; got: %T", v)
	}
	if want, have := "olivere", s; want != have {
		t.Fatalf("expected to find bucket key value %q; got: %q", want, have)
	}
	v, found = bucket.Key["composite_retweets"]
	if !found {
		t.Fatalf("expected to find bucket key %q", "composite_retweets")
	}
	f, ok := v.(float64)
	if !ok {
		t.Fatalf("expected to have bucket key of type string; got: %T", v)
	}
	if want, have := 0.0, f; want != have {
		t.Fatalf("expected to find bucket key value %v; got: %v", want, have)
	}
	v, found = bucket.Key["composite_created"]
	if !found {
		t.Fatalf("expected to find bucket key %q", "composite_created")
	}
	f, ok = v.(float64)
	if !ok {
		t.Fatalf("expected to have bucket key of type string; got: %T", v)
	}
	if want, have := 1349856720000.0, f; want != have {
		t.Fatalf("expected to find bucket key value %v; got: %v", want, have)
	}

	// 2nd bucket
	bucket = agg.Buckets[1]
	if want, have := int64(1), bucket.DocCount; want != have {
		t.Fatalf("expected aggregation bucket doc count = %v; got: %v", want, have)
	}
	if want, have := 3, len(bucket.Key); want != have {
		t.Fatalf("expected aggregation bucket key length = %v; got: %v", want, have)
	}
	v, found = bucket.Key["composite_users"]
	if !found {
		t.Fatalf("expected to find bucket key %q", "composite_users")
	}
	s, ok = v.(string)
	if !ok {
		t.Fatalf("expected to have bucket key of type string; got: %T", v)
	}
	if want, have := "olivere", s; want != have {
		t.Fatalf("expected to find bucket key value %q; got: %q", want, have)
	}
	v, found = bucket.Key["composite_retweets"]
	if !found {
		t.Fatalf("expected to find bucket key %q", "composite_retweets")
	}
	f, ok = v.(float64)
	if !ok {
		t.Fatalf("expected to have bucket key of type string; got: %T", v)
	}
	if want, have := 108.0, f; want != have {
		t.Fatalf("expected to find bucket key value %v; got: %v", want, have)
	}
	v, found = bucket.Key["composite_created"]
	if !found {
		t.Fatalf("expected to find bucket key %q", "composite_created")
	}
	f, ok = v.(float64)
	if !ok {
		t.Fatalf("expected to have bucket key of type string; got: %T", v)
	}
	if want, have := 1355333880000.0, f; want != have {
		t.Fatalf("expected to find bucket key value %v; got: %v", want, have)
	}

	// 3rd bucket
	bucket = agg.Buckets[2]
	if want, have := int64(1), bucket.DocCount; want != have {
		t.Fatalf("expected aggregation bucket doc count = %v; got: %v", want, have)
	}
	if want, have := 3, len(bucket.Key); want != have {
		t.Fatalf("expected aggregation bucket key length = %v; got: %v", want, have)
	}
	v, found = bucket.Key["composite_users"]
	if !found {
		t.Fatalf("expected to find bucket key %q", "composite_users")
	}
	s, ok = v.(string)
	if !ok {
		t.Fatalf("expected to have bucket key of type string; got: %T", v)
	}
	if want, have := "sandrae", s; want != have {
		t.Fatalf("expected to find bucket key value %q; got: %q", want, have)
	}
	v, found = bucket.Key["composite_retweets"]
	if !found {
		t.Fatalf("expected to find bucket key %q", "composite_retweets")
	}
	f, ok = v.(float64)
	if !ok {
		t.Fatalf("expected to have bucket key of type string; got: %T", v)
	}
	if want, have := 12.0, f; want != have {
		t.Fatalf("expected to find bucket key value %v; got: %v", want, have)
	}
	v, found = bucket.Key["composite_created"]
	if !found {
		t.Fatalf("expected to find bucket key %q", "composite_created")
	}
	f, ok = v.(float64)
	if !ok {
		t.Fatalf("expected to have bucket key of type string; got: %T", v)
	}
	if want, have := 1321009080000.0, f; want != have {
		t.Fatalf("expected to find bucket key value %v; got: %v", want, have)
	}
}

func TestAggsScriptedMetric(t *testing.T) {
	s := `{
  "bool_metric": {
    "value": true
  },
  "int_metric": {
    "value": 1
  },
  "float_metric": {
    "value": 2.5
  },
  "string_metric": {
    "value": "test"
  },
  "slice_metric": {
    "value": [
      1,
      2,
      3
    ]
  },
  "map_metric": {
    "value": {
      "a": "1",
      "b": "2"
    }
  }
}`

	aggs := new(Aggregations)
	err := json.Unmarshal([]byte(s), &aggs)
	if err != nil {
		t.Fatalf("expected no error decoding; got: %+v", err)
	}

	agg, found := aggs.ScriptedMetric("bool_metric")
	if !found {
		t.Fatalf("expected aggregation to be found; got: %+v", found)
	}
	if agg == nil {
		t.Fatalf("expected aggregation != nil; got: %+v", agg)
	}
	if v, ok := agg.Value.(bool); !ok || !v {
		t.Fatalf("expected aggregation value is bool true; got: %+v", agg.Value)
	}

	agg, found = aggs.ScriptedMetric("int_metric")
	if !found {
		t.Fatalf("expected aggregation to be found; got: %+v", found)
	}
	if agg == nil {
		t.Fatalf("expected aggregation != nil; got: %+v", agg)
	}
	if v, ok := agg.Value.(json.Number); ok {
		if iv, err := v.Int64(); err != nil || iv != 1 {
			t.Fatalf("expected aggregation value is 1; got: %+v", iv)
		}
	} else {
		t.Fatalf("expected aggregation value is json.Number; got: %+v", agg.Value)
	}

	agg, found = aggs.ScriptedMetric("float_metric")
	if !found {
		t.Fatalf("expected aggregation to be found; got: %+v", found)
	}
	if agg == nil {
		t.Fatalf("expected aggregation != nil; got: %+v", agg)
	}
	if v, ok := agg.Value.(json.Number); ok {
		if iv, err := v.Float64(); err != nil || iv != 2.5 {
			t.Fatalf("expected aggregation value is 2.5; got: %+v", iv)
		}
	} else {
		t.Fatalf("expected aggregation value is json.Number; got: %+v", agg.Value)
	}

	agg, found = aggs.ScriptedMetric("string_metric")
	if !found {
		t.Fatalf("expected aggregation to be found; got: %+v", found)
	}
	if agg == nil {
		t.Fatalf("expected aggregation != nil; got: %+v", agg)
	}
	if v, ok := agg.Value.(string); !ok || v != "test" {
		t.Fatalf("expected aggregation value is test; got: %+v", agg.Value)
	}

	agg, found = aggs.ScriptedMetric("slice_metric")
	if !found {
		t.Fatalf("expected aggregation to be found; got: %+v", found)
	}
	if agg == nil {
		t.Fatalf("expected aggregation != nil; got: %+v", agg)
	}
	if v, ok := agg.Value.([]interface{}); ok {
		expected := []interface{}{json.Number("1"), json.Number("2"), json.Number("3")}
		if !reflect.DeepEqual(v, expected) {
			t.Fatalf("expected %+v; got: %+v", expected, v)
		}
	} else {
		t.Fatalf("expected aggregation value is []interface{}; got: %+v", agg.Value)
	}

	agg, found = aggs.ScriptedMetric("map_metric")
	if !found {
		t.Fatalf("expected aggregation to be found; got: %+v", found)
	}
	if agg == nil {
		t.Fatalf("expected aggregation != nil; got: %+v", agg)
	}
	if v, ok := agg.Value.(map[string]interface{}); ok {
		expected := map[string]interface{}{
			"a": "1",
			"b": "2",
		}
		if !reflect.DeepEqual(v, expected) {
			t.Fatalf("expected %+v; got: %+v", expected, v)
		}
	} else {
		t.Fatalf("expected aggregation value is map[string]interface{}; got: %+v", agg.Value)
	}
}
