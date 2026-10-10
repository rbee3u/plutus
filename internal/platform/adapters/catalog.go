package adapters

// PlatformInfo describes one supported Platform for the API and the UI.
type PlatformInfo struct {
	ID               Platform
	Name             string
	AdapterAvailable bool
	Structures       []StructureInfo
	CredentialForm   CredentialForm
}

// StructureInfo describes one Account Structure, the Account Types it holds, and the Margin
// Modes the Platform may report within it.
type StructureInfo struct {
	ID           AccountStructure
	Name         string
	AccountTypes []AccountTypeInfo
	MarginModes  []MarginModeInfo
}

// AccountTypeInfo describes one Account Type.
type AccountTypeInfo struct {
	ID   AccountType
	Name string
}

// MarginModeInfo describes one Margin Mode.
type MarginModeInfo struct {
	ID   MarginMode
	Name string
}

type platformDescriptor struct {
	id             Platform
	name           string
	structures     []StructureInfo
	credentialForm CredentialForm
}

// platformCatalog is the ordered product catalog, independent of runtime availability.
var platformCatalog = []platformDescriptor{
	{id: PlatformBinance, name: "Binance", credentialForm: binanceCredentialForm(), structures: []StructureInfo{
		{ID: StructureBinanceClassic, Name: "Classic Trading", AccountTypes: []AccountTypeInfo{
			{ID: "spot", Name: "Spot"},
			{ID: "cross_margin", Name: "Cross Margin"},
			{ID: "isolated_margin", Name: "Isolated Margin"},
			{ID: "um_futures", Name: "USDⓈ-M Futures"},
			{ID: "cm_futures", Name: "COIN-M Futures"},
			{ID: "funding", Name: "Funding"},
			{ID: "earn", Name: "Earn"},
			{ID: TypeDualInvestment, Name: "Dual Investment"},
		}},
		{ID: StructureBinancePM, Name: "Portfolio Margin", AccountTypes: []AccountTypeInfo{
			{ID: "spot", Name: "Spot"},
			{ID: "portfolio_margin", Name: "Portfolio Margin"},
			{ID: "funding", Name: "Funding"},
			{ID: "earn", Name: "Earn"},
			{ID: TypeDualInvestment, Name: "Dual Investment"},
		}},
	}},
	{id: PlatformOKX, name: "OKX", credentialForm: okxCredentialForm(), structures: []StructureInfo{
		{
			ID:   StructureOKXUnified,
			Name: "Unified account",
			AccountTypes: []AccountTypeInfo{
				{ID: "trading", Name: "Trading"},
				{ID: "funding", Name: "Funding"},
				{ID: "earn", Name: "Earn"},
				{ID: TypeDualInvestment, Name: "Dual Investment"},
			},
			MarginModes: []MarginModeInfo{
				{ID: MarginOKXSpot, Name: "Spot mode"},
				{ID: MarginOKXFutures, Name: "Futures mode"},
				{ID: MarginOKXMCM, Name: "Multi-currency margin"},
				{ID: MarginOKXPM, Name: "Portfolio margin"},
			},
		},
	}},
	{id: PlatformBybit, name: "Bybit", credentialForm: bybitCredentialForm(), structures: []StructureInfo{
		{
			ID:   StructureBybitUTA2,
			Name: "Unified Trading Account 2.0",
			AccountTypes: []AccountTypeInfo{
				{ID: "unified", Name: "Unified"},
				{ID: "funding", Name: "Funding"},
				{ID: "earn", Name: "Earn"},
				{ID: TypeDualInvestment, Name: "Dual Investment"},
			},
			MarginModes: []MarginModeInfo{
				{ID: MarginBybitIsolated, Name: "Isolated Margin"},
				{ID: MarginBybitCross, Name: "Cross Margin"},
				{ID: MarginBybitPM, Name: "Portfolio Margin"},
			},
		},
	}},
	{id: PlatformKraken, name: "Kraken", credentialForm: krakenCredentialForm(), structures: []StructureInfo{
		{ID: StructureKrakenSeparate, Name: "Separate wallets", AccountTypes: []AccountTypeInfo{
			{ID: "main", Name: "Main"},
			{ID: "multi_m", Name: "Multi-M"},
			{ID: "coin_m", Name: "Coin-M"},
			{ID: "derivatives_holding", Name: "Derivatives Holding"},
			{ID: "earn", Name: "Earn"},
		}},
		{ID: StructureKrakenUnified, Name: "Unified Wallet", AccountTypes: []AccountTypeInfo{
			{ID: "main", Name: "Main"},
			{ID: "coin_m", Name: "Coin-M"},
			{ID: "derivatives_holding", Name: "Derivatives Holding"},
			{ID: "earn", Name: "Earn"},
		}},
	}},
}

// OfferedPlatforms lists the offered Platforms in display order.
func OfferedPlatforms() []Platform {
	ids := make([]Platform, 0, len(platformCatalog))
	for _, spec := range platformCatalog {
		if len(spec.structures) > 0 {
			ids = append(ids, spec.id)
		}
	}
	return ids
}

// Platforms returns independent descriptions of offered products and runtime availability.
func (r *Registry) Platforms() []PlatformInfo {
	out := make([]PlatformInfo, 0, len(platformCatalog))
	for _, spec := range platformCatalog {
		if len(spec.structures) == 0 {
			continue
		}
		info := PlatformInfo{ID: spec.id, Name: spec.name, AdapterAvailable: r.Available(spec.id), CredentialForm: spec.credentialForm.clone()}
		for _, structure := range spec.structures {
			structure.AccountTypes = append([]AccountTypeInfo(nil), structure.AccountTypes...)
			structure.MarginModes = append([]MarginModeInfo(nil), structure.MarginModes...)
			info.Structures = append(info.Structures, structure)
		}
		out = append(out, info)
	}
	return out
}

func platformSpec(id Platform) (platformDescriptor, bool) {
	for _, spec := range platformCatalog {
		if spec.id == id {
			return spec, true
		}
	}
	return platformDescriptor{}, false
}

func structureSpec(id AccountStructure) (StructureInfo, bool) {
	for _, spec := range platformCatalog {
		for _, structure := range spec.structures {
			if structure.ID == id {
				return structure, true
			}
		}
	}
	return StructureInfo{}, false
}

// ValidPlatform reports whether the code is a supported Platform.
func ValidPlatform(platform Platform) bool {
	_, ok := platformSpec(platform)
	return ok
}

// ValidStructure reports whether the Account Structure belongs to the Platform.
func ValidStructure(platform Platform, structure AccountStructure) bool {
	spec, _ := platformSpec(platform)
	for _, s := range spec.structures {
		if s.ID == structure {
			return true
		}
	}
	return false
}

// StructureName returns the display name of an Account Structure.
func StructureName(structure AccountStructure) string {
	if spec, ok := structureSpec(structure); ok {
		return spec.Name
	}
	return string(structure)
}

// AccountTypes returns the Account Types of an Account Structure in display order.
func AccountTypes(structure AccountStructure) []AccountTypeInfo {
	spec, _ := structureSpec(structure)
	return append([]AccountTypeInfo(nil), spec.AccountTypes...)
}

// AccountTypeIDs returns the Account Type codes of an Account Structure in display order.
func AccountTypeIDs(structure AccountStructure) []AccountType {
	types := AccountTypes(structure)
	out := make([]AccountType, 0, len(types))
	for _, t := range types {
		out = append(out, t.ID)
	}
	return out
}

// ValidAccountType reports whether the Account Type belongs to the Account Structure.
func ValidAccountType(structure AccountStructure, accountType AccountType) bool {
	for _, t := range AccountTypes(structure) {
		if t.ID == accountType {
			return true
		}
	}
	return false
}

// MarginModes returns the Margin Modes the Platform may report within an Account Structure.
func MarginModes(structure AccountStructure) []MarginModeInfo {
	spec, _ := structureSpec(structure)
	return append([]MarginModeInfo(nil), spec.MarginModes...)
}

// ValidMarginMode reports whether a Margin Mode belongs to the Account Structure. The empty
// Margin Mode is valid exactly for structures that have none.
func ValidMarginMode(structure AccountStructure, mode MarginMode) bool {
	modes := MarginModes(structure)
	if mode == "" {
		return len(modes) == 0
	}
	for _, m := range modes {
		if m.ID == mode {
			return true
		}
	}
	return false
}

// MarginModeName returns the display name of a Margin Mode.
func MarginModeName(mode MarginMode) string {
	for _, spec := range platformCatalog {
		for _, structure := range spec.structures {
			for _, m := range structure.MarginModes {
				if m.ID == mode {
					return m.Name
				}
			}
		}
	}
	return string(mode)
}

// PlatformName returns the display name of a Platform.
func PlatformName(platform Platform) string {
	if spec, ok := platformSpec(platform); ok {
		return spec.name
	}
	return string(platform)
}

// AccountName is the catalog label shared by configuration and financial views.
func AccountName(accountType AccountType) string {
	if accountType == TypeManual {
		return "Manual"
	}
	for _, platform := range platformCatalog {
		for _, structure := range platform.structures {
			for _, account := range structure.AccountTypes {
				if account.ID == accountType {
					return account.Name
				}
			}
		}
	}
	return string(accountType)
}
