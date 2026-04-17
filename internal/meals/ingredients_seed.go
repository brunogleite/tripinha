package meals

// IBSIngredients returns the canonical seed dictionary of IBS-relevant ingredients
// (top 50 FODMAPs, dairy, and gluten sources). Requires human review before
// normalization produces trustworthy results (see issue #5).
func IBSIngredients() []string {
	return []string{
		// FODMAPs — oligosaccharides (fructans / GOS)
		"Wheat",
		"Rye",
		"Barley",
		"Garlic",
		"Onion",
		"Leek",
		"Shallot",
		"Spring Onion",
		"Chickpeas",
		"Lentils",
		"Kidney Beans",
		"Black Beans",
		"Soybeans",
		"Cashews",
		"Pistachios",

		// FODMAPs — disaccharides (lactose)
		"Milk",
		"Skimmed Milk",
		"Whole Milk",
		"Condensed Milk",
		"Cream",
		"Ice Cream",
		"Yogurt",
		"Soft Cheese",
		"Ricotta",
		"Cottage Cheese",

		// FODMAPs — monosaccharides (excess fructose)
		"Fructose",
		"High Fructose Corn Syrup",
		"Honey",
		"Mango",
		"Apple",
		"Pear",
		"Watermelon",

		// FODMAPs — polyols
		"Sorbitol",
		"Mannitol",
		"Xylitol",
		"Maltitol",
		"Isomalt",
		"Cauliflower",
		"Mushrooms",
		"Avocado",

		// Gluten sources
		"Gluten",
		"Wheat Flour",
		"Spelt",
		"Semolina",
		"Oats",

		// Common dairy
		"Butter",
		"Lactose",
		"Whey",
		"Casein",
	}
}
