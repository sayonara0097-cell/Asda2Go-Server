package db

import "fmt"

type CraftMaterialRow struct {
	ItemID int
	Amount int
}

type CraftResultRow struct {
	ItemID int
	Amount int
}

type CraftRecipeRow struct {
	RecipeID              int
	Name                  string
	RequiredCraftingLevel byte
	Materials             []CraftMaterialRow
	Results               []CraftResultRow
}

func LoadCraftRecipes() ([]CraftRecipeRow, error) {
	rows, err := loadCanonicalCraftRecipes()
	if err != nil {
		return nil, err
	}
	if len(rows) > 0 {
		return rows, nil
	}
	return loadLegacyCraftRecipes()
}

func loadCanonicalCraftRecipes() ([]CraftRecipeRow, error) {
	nameExpr := selectColumnOrDefault("Asda2CraftRecipe", "Name", "''")
	columns := []string{
		"RecipeId",
		nameExpr,
		selectColumnOrDefault("Asda2CraftRecipe", "RequiredCraftingLevel", "0"),
	}
	for i := 1; i <= 6; i++ {
		columns = append(columns,
			selectColumnOrDefault("Asda2CraftRecipe", fmt.Sprintf("Material%dItemId", i), "0"),
			selectColumnOrDefault("Asda2CraftRecipe", fmt.Sprintf("Material%dAmount", i), "0"),
		)
	}
	columns = append(columns,
		selectColumnOrDefault("Asda2CraftRecipe", "ResultItemId", "0"),
		selectColumnOrDefault("Asda2CraftRecipe", "ResultAmount", "0"),
	)
	for i := 2; i <= 7; i++ {
		columns = append(columns,
			selectColumnOrDefault("Asda2CraftRecipe", fmt.Sprintf("Result%dItemId", i), "0"),
			selectColumnOrDefault("Asda2CraftRecipe", fmt.Sprintf("Result%dAmount", i), "0"),
		)
	}
	where := "1 = 1"
	if tableColumnExists("Asda2CraftRecipe", "IsEnabled") {
		where = "IsEnabled = 1"
	}
	query := fmt.Sprintf(`SELECT %s FROM Asda2CraftRecipe WHERE %s`, joinSQLColumns(columns), where)
	rows, err := DB.Query(query)
	if err != nil {
		if missingItemTemplateTable(err, "Asda2CraftRecipe") {
			return nil, nil
		}
		return nil, err
	}
	defer rows.Close()
	return scanCraftRecipes(rows)
}

func loadLegacyCraftRecipes() ([]CraftRecipeRow, error) {
	columns := []string{"Id", "Name", "CraftingLevel"}
	for i := 0; i < 6; i++ {
		columns = append(columns, fmt.Sprintf("RequredItemId_%d", i), fmt.Sprintf("ReqiredItemAmount_%d", i))
	}
	for i := 0; i < 7; i++ {
		columns = append(columns, fmt.Sprintf("ResultItemId_%d", i), fmt.Sprintf("ResultItemAmount_%d", i))
	}
	rows, err := DB.Query(fmt.Sprintf(`SELECT %s FROM CraftRecord`, joinSQLColumns(columns)))
	if err != nil {
		if missingItemTemplateTable(err, "CraftRecord") {
			return nil, nil
		}
		return nil, err
	}
	defer rows.Close()
	return scanCraftRecipes(rows)
}

func scanCraftRecipes(rows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}) ([]CraftRecipeRow, error) {
	out := make([]CraftRecipeRow, 0)
	for rows.Next() {
		var recipeID, level int
		var name string
		materialIDs := make([]int, 6)
		materialAmounts := make([]int, 6)
		resultIDs := make([]int, 7)
		resultAmounts := make([]int, 7)
		dest := []any{&recipeID, &name, &level}
		for i := range materialIDs {
			dest = append(dest, &materialIDs[i], &materialAmounts[i])
		}
		for i := range resultIDs {
			dest = append(dest, &resultIDs[i], &resultAmounts[i])
		}
		if err := rows.Scan(dest...); err != nil {
			return nil, err
		}
		row := CraftRecipeRow{
			RecipeID:              recipeID,
			Name:                  name,
			RequiredCraftingLevel: byteFromDB(level),
			Materials:             normalizeCraftMaterials(materialIDs, materialAmounts),
			Results:               normalizeCraftResults(resultIDs, resultAmounts),
		}
		if row.RecipeID > 0 && len(row.Results) > 0 {
			out = append(out, row)
		}
	}
	return out, rows.Err()
}

func normalizeCraftMaterials(ids []int, amounts []int) []CraftMaterialRow {
	out := make([]CraftMaterialRow, 0, len(ids))
	for i, itemID := range ids {
		if itemID <= 0 {
			continue
		}
		amount := 1
		if i < len(amounts) && amounts[i] > 0 {
			amount = amounts[i]
		}
		out = append(out, CraftMaterialRow{ItemID: itemID, Amount: amount})
	}
	return out
}

func normalizeCraftResults(ids []int, amounts []int) []CraftResultRow {
	out := make([]CraftResultRow, 0, len(ids))
	for i, itemID := range ids {
		if itemID <= 0 {
			continue
		}
		amount := 1
		if i < len(amounts) && amounts[i] > 0 {
			amount = amounts[i]
		}
		out = append(out, CraftResultRow{ItemID: itemID, Amount: amount})
	}
	return out
}

func joinSQLColumns(columns []string) string {
	if len(columns) == 0 {
		return ""
	}
	out := columns[0]
	for _, column := range columns[1:] {
		out += ", " + column
	}
	return out
}
