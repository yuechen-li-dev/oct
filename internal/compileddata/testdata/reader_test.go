package main

import "testing"

func TestGeneratedDataIsDirectlyReadable(t *testing.T) {
	if CatalogDataRowCount != 6 || len(CatalogData) != 6 {
		t.Fatalf("catalog extent = %d/%d", CatalogDataRowCount, len(CatalogData))
	}
	if CatalogData[1].Status != Status_Published || CatalogData[2].Price != 3.25 {
		t.Fatalf("unexpected static row: %#v", CatalogData[2])
	}
	if len(PublishedIDs) != 4 || PublishedIDs[3] != 6 {
		t.Fatalf("unexpected static index: %#v", PublishedIDs)
	}
}

func BenchmarkGeneratedQuery(b *testing.B) {
	var sink PositivePrice
	b.ReportAllocs()
	for index := 0; index < b.N; index++ {
		sink = CatalogData[4].Price
	}
	_ = sink
}
