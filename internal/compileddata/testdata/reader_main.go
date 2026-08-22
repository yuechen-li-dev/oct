package main

import "fmt"

func main() {
	fmt.Printf("rows=%d selected=%d price=%.2f\n", CatalogDataRowCount, PublishedIDs[2], CatalogData[4].Price)
}
