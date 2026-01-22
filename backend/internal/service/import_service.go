package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"

	"homexai/internal/models/master"
	"homexai/internal/models/property"

	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
)

// ImportService handles Excel import operations
type ImportService struct {
	masterDB            *gorm.DB
	verificationService *VerificationService
	smtpService         *SmtpService
}

// NewImportService creates a new import service
func NewImportService(masterDB *gorm.DB, verificationService *VerificationService, smtpService *SmtpService) *ImportService {
	return &ImportService{
		masterDB:            masterDB,
		verificationService: verificationService,
		smtpService:         smtpService,
	}
}

// ImportError represents an import error with code and details
type ImportError struct {
	Code    string                 `json:"code"`
	Message string                 `json:"message"`
	Details map[string]interface{} `json:"details,omitempty"`
}

func (e *ImportError) Error() string {
	return e.Message
}

// Helper functions for pointer conversion
func strPtrImport(s string) *string {
	return &s
}

func intPtrImport(i int) *int {
	return &i
}

// ImportResult represents the result of an import operation
type ImportResult struct {
	TaskID        uint     `json:"task_id"`
	TotalRows     int      `json:"total_rows"`
	ProcessedRows int      `json:"processed_rows"`
	UnitsCreated  int      `json:"units_created"`
	UnitsUpdated  int      `json:"units_updated"`
	UsersCreated  int      `json:"users_created"`
	EmailsSent    int      `json:"emails_sent"`
	FailedRows    int      `json:"failed_rows"`
	Errors        []string `json:"errors,omitempty"`
}

// ExcelRow represents a single row from the Excel file
type ExcelRow struct {
	RowNum        int
	Tower         string
	Floor         int
	Unit          string
	Area          float64
	Type          string
	OwnerName     string
	OwnerContact  string
	OwnerEmail    string
	SPAName       string
	SPAContact    string
	SPAEmail      string
	TenantName    string
	TenantContact string
	TenantEmail   string
	MoveIn        *time.Time
	MoveOut       *time.Time
}

// PersonInfo represents person information
type PersonInfo struct {
	Name    string
	Contact string
	Email   string
}

// TenantInfo represents tenant information with lease dates
type TenantInfo struct {
	PersonInfo
	MoveIn  *time.Time
	MoveOut *time.Time
}

// UnitImportData represents grouped data for a single unit
type UnitImportData struct {
	UnitKey  string
	Tower    string
	Floor    int
	UnitNum  string
	Area     float64
	Type     string
	Bedrooms int
	Owners   []PersonInfo
	SPAs     []PersonInfo
	Tenants  []TenantInfo
}

// ValidateSheetName validates that Excel has a sheet matching property name (case-insensitive)
func (s *ImportService) ValidateSheetName(f *excelize.File, propertyName string) (string, error) {
	sheets := f.GetSheetList()

	if len(sheets) == 0 {
		return "", &ImportError{
			Code:    "NO_SHEETS",
			Message: "Excel file has no sheets",
		}
	}

	propertyNameLower := strings.ToLower(strings.TrimSpace(propertyName))

	for _, sheetName := range sheets {
		if strings.ToLower(strings.TrimSpace(sheetName)) == propertyNameLower {
			return sheetName, nil
		}
	}

	return "", &ImportError{
		Code:    "SHEET_NAME_MISMATCH",
		Message: fmt.Sprintf("Sheet name mismatch: expected '%s' (case-insensitive), but found: %v", propertyName, sheets),
		Details: map[string]interface{}{
			"expected": propertyName,
			"found":    sheets,
		},
	}
}

// ValidateFileType validates that the uploaded file matches the expected import type
// Returns: fileType ("units", "parking", "unknown"), isValid (bool), error
func (s *ImportService) ValidateFileType(f *excelize.File, expectedType string) (string, bool, error) {
	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return "unknown", false, &ImportError{
			Code:    "NO_SHEETS",
			Message: "Excel file has no sheets",
		}
	}

	// Read first sheet
	firstSheet := sheets[0]
	rows, err := f.GetRows(firstSheet)
	if err != nil {
		return "unknown", false, fmt.Errorf("failed to read sheet: %w", err)
	}

	if len(rows) < 1 {
		return "unknown", false, &ImportError{
			Code:    "NO_DATA",
			Message: "Excel file has no data",
		}
	}

	// Get headers
	headers := rows[0]
	normalizedHeaders := make([]string, len(headers))
	for i, h := range headers {
		normalizedHeaders[i] = strings.ToLower(strings.TrimSpace(h))
	}

	// Check for Units columns
	hasTower := false
	hasFloor := false
	hasUnit := false
	hasArea := false
	hasType := false
	hasSlotNumber := false

	for _, h := range normalizedHeaders {
		if strings.Contains(h, "tower") || strings.Contains(h, "building") {
			hasTower = true
		}
		if strings.Contains(h, "floor") || strings.Contains(h, "level") {
			hasFloor = true
		}
		if strings.Contains(h, "unit") && !strings.Contains(h, "parking") {
			hasUnit = true
		}
		if strings.Contains(h, "area") || strings.Contains(h, "size") {
			hasArea = true
		}
		if strings.Contains(h, "type") && !strings.Contains(h, "slot") {
			hasType = true
		}
		if strings.Contains(h, "slot") && (strings.Contains(h, "number") || strings.Contains(h, "num")) {
			hasSlotNumber = true
		}
	}

	// Check data characteristics if we have sample rows
	var hasUnitsTypeValues, hasParkingTypeValues bool
	if len(rows) > 1 && hasType {
		typeColIndex := -1
		for i, h := range normalizedHeaders {
			if strings.Contains(h, "type") && !strings.Contains(h, "slot") {
				typeColIndex = i
				break
			}
		}

		if typeColIndex >= 0 {
			unitsTypes := []string{"studio", "1bedroom", "2bedroom", "3bedroom", "penthouse", "park slot"}
			parkingTypes := []string{"indoor", "outdoor", "covered", "uncovered"}

			for i := 1; i < len(rows) && i <= 10; i++ { // Check first 10 rows
				if typeColIndex < len(rows[i]) {
					cellValue := strings.ToLower(strings.TrimSpace(rows[i][typeColIndex]))
					for _, ut := range unitsTypes {
						if strings.Contains(cellValue, ut) || strings.Contains(ut, cellValue) {
							hasUnitsTypeValues = true
							break
						}
					}
					for _, pt := range parkingTypes {
						if strings.Contains(cellValue, pt) || strings.Contains(pt, cellValue) {
							hasParkingTypeValues = true
							break
						}
					}
				}
			}
		}
	}

	// Determine file type
	detectedType := "unknown"
	unitsScore := 0
	parkingScore := 0

	if hasTower {
		unitsScore++
	}
	if hasFloor {
		unitsScore++
	}
	if hasUnit {
		unitsScore++
	}
	if hasArea {
		unitsScore++
	}
	if hasType {
		unitsScore++
	}
	if hasUnitsTypeValues {
		unitsScore += 2
	}

	if hasSlotNumber {
		parkingScore += 3
	}
	if hasParkingTypeValues {
		parkingScore += 2
	}

	if unitsScore > parkingScore && unitsScore >= 3 {
		detectedType = "units"
	} else if parkingScore > unitsScore && parkingScore >= 2 {
		detectedType = "parking"
	} else if hasSlotNumber {
		detectedType = "parking"
	} else if hasTower && hasFloor && hasUnit {
		detectedType = "units"
	}

	// Validate against expected type
	isValid := false
	if expectedType == "units" {
		isValid = detectedType == "units" || (detectedType == "unknown" && unitsScore >= 3)
		if detectedType == "parking" {
			return detectedType, false, &ImportError{
				Code:    "FILE_TYPE_MISMATCH",
				Message: "This file appears to be a Parking import file. Please use the Parking import page.",
				Details: map[string]interface{}{
					"expected": "units",
					"detected": detectedType,
				},
			}
		}
	} else if expectedType == "parking" {
		isValid = detectedType == "parking" || (detectedType == "unknown" && parkingScore >= 2)
		if detectedType == "units" {
			return detectedType, false, &ImportError{
				Code:    "FILE_TYPE_MISMATCH",
				Message: "This file appears to be a Units import file. Please use the Units import page.",
				Details: map[string]interface{}{
					"expected": "parking",
					"detected": detectedType,
				},
			}
		}
	}

	if !isValid && detectedType != "unknown" {
		return detectedType, false, &ImportError{
			Code:    "FILE_TYPE_MISMATCH",
			Message: fmt.Sprintf("File type mismatch: expected %s, detected %s", expectedType, detectedType),
			Details: map[string]interface{}{
				"expected": expectedType,
				"detected": detectedType,
			},
		}
	}

	return detectedType, isValid, nil
}

// ImportUnitsFromExcel imports units from an Excel file
func (s *ImportService) ImportUnitsFromExcel(
	ctx context.Context,
	propertyID uint,
	propertyName string,
	propertyDB *gorm.DB,
	reader io.Reader,
) (*ImportResult, error) {
	f, err := excelize.OpenReader(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to open excel file: %w", err)
	}
	defer f.Close()

	// Validate sheet name
	actualSheetName, err := s.ValidateSheetName(f, propertyName)
	if err != nil {
		return nil, err
	}

	// Read rows from the matched sheet
	rows, err := f.GetRows(actualSheetName)
	if err != nil {
		return nil, fmt.Errorf("failed to read sheet '%s': %w", actualSheetName, err)
	}

	if len(rows) < 2 {
		return nil, &ImportError{
			Code:    "NO_DATA",
			Message: "Excel file has no data rows",
		}
	}

	// Parse and process rows
	return s.processRows(ctx, propertyID, propertyDB, rows)
}

// processRows processes Excel rows and imports data
func (s *ImportService) processRows(
	ctx context.Context,
	propertyID uint,
	propertyDB *gorm.DB,
	rows [][]string,
) (*ImportResult, error) {
	result := &ImportResult{}

	// Parse header row to get column indices
	header := rows[0]
	colIdx := s.parseHeader(header)

	// Parse data rows
	var excelRows []ExcelRow
	for i := 1; i < len(rows); i++ {
		row := rows[i]
		if !s.isValidDataRow(row, colIdx) {
			continue
		}

		excelRow, err := s.parseRow(row, colIdx, i+1)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("Row %d: %v", i+1, err))
			result.FailedRows++
			continue
		}
		excelRows = append(excelRows, *excelRow)
		result.TotalRows++
	}

	// Group by unit
	unitDataMap := s.groupByUnit(excelRows)

	// Process each unit
	for _, unitData := range unitDataMap {
		err := s.processUnit(ctx, propertyID, propertyDB, unitData, result)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("Unit %s: %v", unitData.UnitKey, err))
		}
		result.ProcessedRows++
	}

	return result, nil
}

// parseHeader parses the header row to get column indices
func (s *ImportService) parseHeader(header []string) map[string]int {
	colIdx := make(map[string]int)
	for i, col := range header {
		colLower := strings.ToLower(strings.TrimSpace(col))
		switch colLower {
		case "tower":
			colIdx["tower"] = i
		case "floor":
			colIdx["floor"] = i
		case "unit":
			colIdx["unit"] = i
		case "area":
			colIdx["area"] = i
		case "type":
			colIdx["type"] = i
		case "owner name":
			colIdx["owner_name"] = i
		case "owner contact":
			colIdx["owner_contact"] = i
		case "owner email":
			colIdx["owner_email"] = i
		case "spa name":
			colIdx["spa_name"] = i
		case "spa contact":
			colIdx["spa_contact"] = i
		case "spa email":
			colIdx["spa_email"] = i
		case "tenant name":
			colIdx["tenant_name"] = i
		case "tenant contact":
			colIdx["tenant_contact"] = i
		case "tenant email":
			colIdx["tenant_email"] = i
		case "move in":
			colIdx["move_in"] = i
		case "move out":
			colIdx["move_out"] = i
		}
	}
	return colIdx
}

// isValidDataRow checks if a row contains valid data
func (s *ImportService) isValidDataRow(row []string, colIdx map[string]int) bool {
	if len(row) < 5 {
		return false
	}

	towerIdx, hasTower := colIdx["tower"]
	floorIdx, hasFloor := colIdx["floor"]
	unitIdx, hasUnit := colIdx["unit"]

	if !hasTower || !hasFloor || !hasUnit {
		return false
	}

	tower := s.getCell(row, towerIdx)
	floor := s.getCell(row, floorIdx)
	unit := s.getCell(row, unitIdx)

	if tower == "" || floor == "" || unit == "" {
		return false
	}

	// Floor must be a number
	if _, err := strconv.Atoi(floor); err != nil {
		return false
	}

	// Exclude instruction rows
	lowerTower := strings.ToLower(tower)
	if strings.Contains(lowerTower, "填表") ||
		strings.Contains(lowerTower, "说明") ||
		strings.Contains(lowerTower, "注意") ||
		strings.Contains(lowerTower, "instruction") {
		return false
	}

	return true
}

// parseRow parses a single row into ExcelRow struct
func (s *ImportService) parseRow(row []string, colIdx map[string]int, rowNum int) (*ExcelRow, error) {
	floorStr := s.getCell(row, colIdx["floor"])
	floor, _ := strconv.Atoi(floorStr)

	areaStr := s.getCell(row, colIdx["area"])
	area, _ := strconv.ParseFloat(areaStr, 64)

	moveIn := s.parseDate(s.getCell(row, colIdx["move_in"]))
	moveOut := s.parseDate(s.getCell(row, colIdx["move_out"]))

	return &ExcelRow{
		RowNum:        rowNum,
		Tower:         s.getCell(row, colIdx["tower"]),
		Floor:         floor,
		Unit:          s.getCell(row, colIdx["unit"]),
		Area:          area,
		Type:          s.getCell(row, colIdx["type"]),
		OwnerName:     s.getCell(row, colIdx["owner_name"]),
		OwnerContact:  s.getCell(row, colIdx["owner_contact"]),
		OwnerEmail:    s.getCell(row, colIdx["owner_email"]),
		SPAName:       s.getCell(row, colIdx["spa_name"]),
		SPAContact:    s.getCell(row, colIdx["spa_contact"]),
		SPAEmail:      s.getCell(row, colIdx["spa_email"]),
		TenantName:    s.getCell(row, colIdx["tenant_name"]),
		TenantContact: s.getCell(row, colIdx["tenant_contact"]),
		TenantEmail:   s.getCell(row, colIdx["tenant_email"]),
		MoveIn:        moveIn,
		MoveOut:       moveOut,
	}, nil
}

// getCell safely gets a cell value from a row
func (s *ImportService) getCell(row []string, idx int) string {
	if idx < 0 || idx >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[idx])
}

// parseDate parses a date string into time.Time
func (s *ImportService) parseDate(dateStr string) *time.Time {
	dateStr = strings.TrimSpace(dateStr)
	if dateStr == "" {
		return nil
	}

	formats := []string{
		"2006/01/02",
		"2006-01-02",
		"01/02/2006",
		"02/01/2006",
		"2006/1/2",
		"2006-1-2",
		"1/2/2006",
		"2/1/2006",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, dateStr); err == nil {
			return &t
		}
	}

	return nil
}

// parsePersonName parses a person name (takes first name if comma-separated)
func (s *ImportService) parsePersonName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}

	if idx := strings.Index(name, ","); idx > 0 {
		name = strings.TrimSpace(name[:idx])
	}

	return name
}

// mapUnitType maps unit type string to bedrooms count
func (s *ImportService) mapUnitType(unitType string) (string, int) {
	t := strings.ToLower(strings.TrimSpace(unitType))

	switch t {
	case "studio":
		return "apartment", 0
	case "1bedroom", "1br", "1 bedroom":
		return "apartment", 1
	case "2bedroom", "2br", "2 bedroom":
		return "apartment", 2
	case "3bedroom", "3br", "3 bedroom":
		return "apartment", 3
	default:
		re := regexp.MustCompile(`(\d+)`)
		if matches := re.FindStringSubmatch(t); len(matches) > 1 {
			bedrooms, _ := strconv.Atoi(matches[1])
			return "apartment", bedrooms
		}
		return "apartment", 0
	}
}

// groupByUnit groups Excel rows by unit key
func (s *ImportService) groupByUnit(rows []ExcelRow) map[string]*UnitImportData {
	unitMap := make(map[string]*UnitImportData)

	for _, row := range rows {
		unitKey := fmt.Sprintf("%s-%d-%s", row.Tower, row.Floor, row.Unit)

		if _, exists := unitMap[unitKey]; !exists {
			unitType, bedrooms := s.mapUnitType(row.Type)
			unitMap[unitKey] = &UnitImportData{
				UnitKey:  unitKey,
				Tower:    row.Tower,
				Floor:    row.Floor,
				UnitNum:  row.Unit,
				Area:     row.Area,
				Type:     unitType,
				Bedrooms: bedrooms,
				Owners:   []PersonInfo{},
				SPAs:     []PersonInfo{},
				Tenants:  []TenantInfo{},
			}
		}

		unitData := unitMap[unitKey]

		// Add owner
		ownerName := s.parsePersonName(row.OwnerName)
		if ownerName != "" {
			owner := PersonInfo{
				Name:    ownerName,
				Contact: row.OwnerContact,
				Email:   strings.ToLower(strings.TrimSpace(row.OwnerEmail)),
			}
			if !s.personExists(unitData.Owners, owner) {
				unitData.Owners = append(unitData.Owners, owner)
			}
		}

		// Add SPA
		spaName := s.parsePersonName(row.SPAName)
		if spaName != "" {
			spa := PersonInfo{
				Name:    spaName,
				Contact: row.SPAContact,
				Email:   strings.ToLower(strings.TrimSpace(row.SPAEmail)),
			}
			if !s.personExists(unitData.SPAs, spa) {
				unitData.SPAs = append(unitData.SPAs, spa)
			}
		}

		// Add tenant
		tenantName := s.parsePersonName(row.TenantName)
		if tenantName != "" {
			tenant := TenantInfo{
				PersonInfo: PersonInfo{
					Name:    tenantName,
					Contact: row.TenantContact,
					Email:   strings.ToLower(strings.TrimSpace(row.TenantEmail)),
				},
				MoveIn:  row.MoveIn,
				MoveOut: row.MoveOut,
			}
			if !s.tenantExists(unitData.Tenants, tenant) {
				unitData.Tenants = append(unitData.Tenants, tenant)
			}
		}
	}

	return unitMap
}

// personExists checks if a person already exists in the list
func (s *ImportService) personExists(persons []PersonInfo, person PersonInfo) bool {
	for _, p := range persons {
		if person.Email != "" && p.Email == person.Email {
			return true
		}
		if person.Email == "" && strings.EqualFold(p.Name, person.Name) {
			return true
		}
	}
	return false
}

// tenantExists checks if a tenant already exists in the list
func (s *ImportService) tenantExists(tenants []TenantInfo, tenant TenantInfo) bool {
	for _, t := range tenants {
		if tenant.Email != "" && t.Email == tenant.Email {
			return true
		}
		if tenant.Email == "" && strings.EqualFold(t.Name, tenant.Name) {
			return true
		}
	}
	return false
}

// processUnit processes a single unit and its related persons
func (s *ImportService) processUnit(
	ctx context.Context,
	propertyID uint,
	propertyDB *gorm.DB,
	unitData *UnitImportData,
	result *ImportResult,
) error {
	if propertyDB == nil {
		return fmt.Errorf("property database connection is nil")
	}
	
	if unitData == nil {
		return fmt.Errorf("unit data is nil")
	}
	
	// Create or update unit
	unitNumber := fmt.Sprintf("%s-%s", unitData.Tower, unitData.UnitNum)
	var unit property.Unit

	err := propertyDB.Where("unit_number = ? AND unit_type = ?", unitNumber, unitData.Type).First(&unit).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Create new unit - use helper functions for pointer conversion
			unit = property.Unit{
				UnitNumber: unitNumber,
				UnitType:   unitData.Type,
				Floor:      intPtrImport(unitData.Floor),
				Building:   strPtrImport(unitData.Tower),
				Area:       strPtrImport(fmt.Sprintf("%.2f", unitData.Area)),
				Bedrooms:   intPtrImport(unitData.Bedrooms),
				Status:     "available",
			}
			if err := propertyDB.Create(&unit).Error; err != nil {
				return fmt.Errorf("failed to create unit: %w", err)
			}
			result.UnitsCreated++
		} else {
			return fmt.Errorf("failed to query unit: %w", err)
		}
	} else {
		// Update existing unit - convert to pointer types
		areaStr := fmt.Sprintf("%.2f", unitData.Area)
		updates := map[string]interface{}{
			"floor":    unitData.Floor,
			"building": unitData.Tower,
			"area":     areaStr,
			"bedrooms": unitData.Bedrooms,
		}
		if err := propertyDB.Model(&unit).Updates(updates).Error; err != nil {
			return fmt.Errorf("failed to update unit: %w", err)
		}
		result.UnitsUpdated++
	}

	// Process owners
	var firstLandlordID *uint
	for _, owner := range unitData.Owners {
		userID, created, err := s.findOrCreateUser(ctx, propertyID, owner, master.RoleLandlord)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("Owner %s: %v", owner.Name, err))
			continue
		}
		if created {
			result.UsersCreated++
		}

		// Create landlord association
		landlordID, err := s.createLandlord(propertyDB, unit.ID, userID)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("Landlord %s: %v", owner.Name, err))
			continue
		}
		if firstLandlordID == nil {
			firstLandlordID = &landlordID
		}

		// Send invitation email if has email
		if owner.Email != "" {
			if err := s.sendInvitationEmail(ctx, owner.Email, owner.Name, "Owner"); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("Email to %s: %v", owner.Email, err))
			} else {
				result.EmailsSent++
			}
		}
	}

	// Process SPAs
	for _, spa := range unitData.SPAs {
		userID, created, err := s.findOrCreateUser(ctx, propertyID, spa, master.RoleSPA)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("SPA %s: %v", spa.Name, err))
			continue
		}
		if created {
			result.UsersCreated++
		}

		// Create SPA association (linked to first landlord)
		if err := s.createSPA(propertyDB, unit.ID, userID, firstLandlordID); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("SPA record %s: %v", spa.Name, err))
			continue
		}

		// Send invitation email if has email
		if spa.Email != "" {
			if err := s.sendInvitationEmail(ctx, spa.Email, spa.Name, "SPA"); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("Email to %s: %v", spa.Email, err))
			} else {
				result.EmailsSent++
			}
		}
	}

	// Process tenants
	for _, tenant := range unitData.Tenants {
		userID, created, err := s.findOrCreateUser(ctx, propertyID, tenant.PersonInfo, master.RoleTenant)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("Tenant %s: %v", tenant.Name, err))
			continue
		}
		if created {
			result.UsersCreated++
		}

		// Create tenant association with lease dates
		if err := s.createTenant(propertyDB, unit.ID, userID, tenant.MoveIn, tenant.MoveOut); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("Tenant record %s: %v", tenant.Name, err))
			continue
		}

		// Send invitation email if has email
		if tenant.Email != "" {
			if err := s.sendInvitationEmail(ctx, tenant.Email, tenant.Name, "Tenant"); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("Email to %s: %v", tenant.Email, err))
			} else {
				result.EmailsSent++
			}
		}
	}

	// Update unit status if has tenants
	if len(unitData.Tenants) > 0 {
		propertyDB.Model(&unit).Update("status", "occupied")
	}

	return nil
}

// findOrCreateUser finds or creates a user and assigns role
func (s *ImportService) findOrCreateUser(
	ctx context.Context,
	propertyID uint,
	person PersonInfo,
	role string,
) (uint, bool, error) {
	var user master.User
	var created bool
	var found bool

	// Clean up contact (phone number)
	phone := strings.TrimSpace(person.Contact)
	email := strings.ToLower(strings.TrimSpace(person.Email))

	// Step 1: Try to find existing user by email
	if email != "" {
		err := s.masterDB.Where("email = ?", email).First(&user).Error
		if err == nil {
			found = true
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, false, fmt.Errorf("failed to query user by email: %w", err)
		}
	}

	// Step 2: If not found by email, try to find by phone
	if !found && phone != "" {
		err := s.masterDB.Where("phone = ?", phone).First(&user).Error
		if err == nil {
			found = true
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, false, fmt.Errorf("failed to query user by phone: %w", err)
		}
	}

	// Step 3: If user not found, create new user
	if !found {
		user = master.User{
			FullName:               person.Name,
			Status:                 "active",
			MustChangePassword:     true,
			PublicEmail:            true,
			PublicPhone:            true,
			PublicFullName:         true,
			PublicPropertyCert:      true,
			PublicVehicleCROR:       true,
			EmailNotificationEnabled: true,
		}

		// Only set email if provided (use helper function for pointer)
		if email != "" {
			user.Email = strPtrImport(email)
			user.EmailVerified = false
		}

		// Only set phone if provided (use helper function for pointer)
		if phone != "" {
			user.Phone = strPtrImport(phone)
		}

		if err := s.masterDB.Create(&user).Error; err != nil {
			return 0, false, fmt.Errorf("failed to create user: %w", err)
		}
		created = true
	} else {
		// User found - optionally update missing fields
		updates := make(map[string]interface{})
		if user.FullName == "" && person.Name != "" {
			updates["full_name"] = person.Name
		}
		if user.Phone == nil && phone != "" {
			updates["phone"] = phone
		}
		if user.Email == nil && email != "" {
			updates["email"] = email
		}
		if len(updates) > 0 {
			s.masterDB.Model(&user).Updates(updates)
		}
	}

	// Assign role to user
	if err := s.assignRole(propertyID, user.ID, role); err != nil {
		// Role might already exist, not a fatal error
		errMsg := err.Error()
		if !strings.Contains(errMsg, "Duplicate") && !strings.Contains(errMsg, "duplicate") {
			return user.ID, created, fmt.Errorf("failed to assign role: %w", err)
		}
		// Duplicate role is not a fatal error, continue
	}

	return user.ID, created, nil
}

// assignRole assigns a role to a user for a property
// Uses FirstOrCreate to avoid duplicate key errors
func (s *ImportService) assignRole(propertyID, userID uint, role string) error {
	// Check if role already exists
	var existing master.UserRole
	err := s.masterDB.Where("user_id = ? AND property_id = ? AND role = ?", userID, propertyID, role).First(&existing).Error
	if err == nil {
		// Role already exists, update status to active if needed
		if existing.Status != "active" {
			return s.masterDB.Model(&existing).Update("status", "active").Error
		}
		return nil // Already exists and is active
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("failed to check existing role: %w", err)
	}

	// Role doesn't exist, create it
	upr := master.UserRole{
		UserID:     userID,
		PropertyID: propertyID,
		Role:       role,
		Status:     "active",
	}

	return s.masterDB.Create(&upr).Error
}

// createLandlord creates a landlord association
func (s *ImportService) createLandlord(propertyDB *gorm.DB, unitID, userID uint) (uint, error) {
	if propertyDB == nil {
		return 0, fmt.Errorf("property database connection is nil")
	}
	
	if unitID == 0 {
		return 0, fmt.Errorf("unit ID is zero")
	}
	
	if userID == 0 {
		return 0, fmt.Errorf("user ID is zero")
	}
	
	var existing property.Landlord
	err := propertyDB.Where("unit_id = ? AND user_id = ?", unitID, userID).First(&existing).Error
	if err == nil {
		return existing.ID, nil // Already exists
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, fmt.Errorf("failed to check existing landlord: %w", err)
	}

	now := time.Now()
	landlord := property.Landlord{
		UnitID:             unitID,
		UserID:             userID,
		OwnershipType:      "full",
		OwnershipStartDate: &now,
	}

	if err := propertyDB.Create(&landlord).Error; err != nil {
		return 0, fmt.Errorf("failed to create landlord: %w", err)
	}

	if landlord.ID == 0 {
		return 0, fmt.Errorf("landlord ID is zero after creation")
	}

	return landlord.ID, nil
}

// createSPA creates a SPA-Unit association directly using SpaUserID
// In the new model, there's no separate SPA table - SPAUnit directly references the SPA user
func (s *ImportService) createSPA(propertyDB *gorm.DB, unitID, spaUserID uint, landlordUserID *uint) error {
	if propertyDB == nil {
		return fmt.Errorf("property database connection is nil")
	}
	
	if unitID == 0 {
		return fmt.Errorf("unit ID is zero")
	}
	
	if spaUserID == 0 {
		return fmt.Errorf("SPA user ID is zero")
	}
	
	// Check if SPA-Unit association already exists
	var existingSPAUnit property.SPAUnit
	err := propertyDB.Where("spa_user_id = ? AND unit_id = ?", spaUserID, unitID).First(&existingSPAUnit).Error
	if err == nil {
		return nil // Already exists
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("failed to check existing SPA unit: %w", err)
	}

	// Create SPA-Unit association
	now := time.Now()
	endDate := now.AddDate(1, 0, 0) // Default 1 year authorization

	spaUnit := property.SPAUnit{
		SpaUserID:              spaUserID,
		UnitID:                 unitID,
		AuthorizationStartDate: &now,
		AuthorizationEndDate:   &endDate,
		Scope:                  "full",
		IsActive:               true,
	}

	return propertyDB.Create(&spaUnit).Error
}

// createTenant creates a tenant association with lease dates
func (s *ImportService) createTenant(propertyDB *gorm.DB, unitID, userID uint, moveIn, moveOut *time.Time) error {
	if propertyDB == nil {
		return fmt.Errorf("property database connection is nil")
	}
	
	if unitID == 0 {
		return fmt.Errorf("unit ID is zero")
	}
	
	if userID == 0 {
		return fmt.Errorf("user ID is zero")
	}
	
	var existing property.Tenant
	err := propertyDB.Where("unit_id = ? AND user_id = ?", unitID, userID).First(&existing).Error
	if err == nil {
		return nil // Already exists
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("failed to check existing tenant: %w", err)
	}

	startDate, endDate := s.resolveLeaseDates(moveIn, moveOut)

	tenant := property.Tenant{
		UnitID:         unitID,
		UserID:         userID,
		LeaseStartDate: startDate,
		LeaseEndDate:   endDate,
		MonthlyRent:    "0",
		Status:         "active",
	}

	return propertyDB.Create(&tenant).Error
}

// resolveLeaseDates resolves lease dates with default values
func (s *ImportService) resolveLeaseDates(moveIn, moveOut *time.Time) (time.Time, time.Time) {
	now := time.Now()

	var startDate, endDate time.Time

	switch {
	case moveIn != nil && moveOut != nil:
		startDate = *moveIn
		endDate = *moveOut
	case moveIn != nil && moveOut == nil:
		startDate = *moveIn
		endDate = moveIn.AddDate(0, 1, 0)
	case moveIn == nil && moveOut != nil:
		startDate = now
		endDate = *moveOut
	default:
		startDate = now
		endDate = now.AddDate(0, 1, 0)
	}

	return startDate, endDate
}

// sendInvitationEmail sends an invitation email with verification code
func (s *ImportService) sendInvitationEmail(ctx context.Context, email, name, role string) error {
	// Generate 24-hour invitation code
	code, err := s.verificationService.GenerateInvitationCode(ctx, email)
	if err != nil {
		return err
	}

	// Send email via SMTP service
	if s.smtpService != nil {
		return s.smtpService.SendInvitationEmail(email, name, code, role)
	}

	// Fallback: log to console
	fmt.Printf("📧 Invitation code for %s (%s) [%s]: %s\n", email, name, role, code)
	return nil
}

// GenerateTemplate generates an Excel template for property data import
func (s *ImportService) GenerateTemplate(propertyName string) ([]byte, error) {
	f := excelize.NewFile()

	// Use property name as sheet name (or default)
	sheetName := "{subdomain}" //subdomain
	if sheetName == "" {
		sheetName = "Property"
	}

	// Rename default sheet
	f.SetSheetName("Sheet1", sheetName)

	// Define headers
	headers := []string{
		"Tower", "Floor", "Unit", "Area", "Type",
		"Owner Name", "Owner Contact", "Owner Email",
		"SPA Name", "SPA Contact", "SPA Email",
		"Tenant Name", "Tenant Contact", "Tenant Email",
		"Move in", "move out",
	}

	// Write headers with style
	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Bold:  true,
			Color: "#FFFFFF",
			Size:  11,
		},
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{"#4F46E5"},
			Pattern: 1,
		},
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
		},
		Border: []excelize.Border{
			{Type: "left", Color: "#000000", Style: 1},
			{Type: "right", Color: "#000000", Style: 1},
			{Type: "top", Color: "#000000", Style: 1},
			{Type: "bottom", Color: "#000000", Style: 1},
		},
	})

	for i, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheetName, cell, header)
		f.SetCellStyle(sheetName, cell, cell, headerStyle)
	}

	// Set column widths
	columnWidths := map[string]float64{
		"A": 8,  // Tower
		"B": 8,  // Floor
		"C": 10, // Unit
		"D": 10, // Area
		"E": 12, // Type
		"F": 18, // Owner Name
		"G": 15, // Owner Contact
		"H": 25, // Owner Email
		"I": 18, // SPA Name
		"J": 15, // SPA Contact
		"K": 25, // SPA Email
		"L": 18, // Tenant Name
		"M": 15, // Tenant Contact
		"N": 25, // Tenant Email
		"O": 12, // Move in
		"P": 12, // move out
	}
	for col, width := range columnWidths {
		f.SetColWidth(sheetName, col, col, width)
	}

	// Add sample data rows
	sampleData := [][]interface{}{
		{"T1", 5, "511", 21.5, "studio", "Albert Wang", "13851410501", "albert@example.com", "Natasha Bayot", "09277057678", "natasha@example.com", "Sky Lucas", "09562345346", "sky@example.com", "2024/10/12", "2025/09/11"},
		{"T1", 5, "511", 21.5, "studio", "Edison Lee", "1234566889", "edison@example.com", "Natasha Bayot", "09277057678", "natasha@example.com", "Allan Sandes", "09562345347", "allan@example.com", "", ""},
		{"T2", 8, "822", 34.6, "1bedroom", "Edison Lee", "1234566889", "edison@example.com", "", "", "", "Ann Javier", "09562345347", "ann@example.com", "", ""},
		{"T3", 26, "2633", 52.5, "2bedroom", "Natasha Bayot", "4355667990", "natasha@example.com", "", "", "", "", "", "", "", ""},
	}

	// Data row style
	dataStyle, _ := f.NewStyle(&excelize.Style{
		Border: []excelize.Border{
			{Type: "left", Color: "#E5E7EB", Style: 1},
			{Type: "right", Color: "#E5E7EB", Style: 1},
			{Type: "top", Color: "#E5E7EB", Style: 1},
			{Type: "bottom", Color: "#E5E7EB", Style: 1},
		},
	})

	for rowIdx, row := range sampleData {
		for colIdx, value := range row {
			cell, _ := excelize.CoordinatesToCellName(colIdx+1, rowIdx+2)
			f.SetCellValue(sheetName, cell, value)
			f.SetCellStyle(sheetName, cell, cell, dataStyle)
		}
	}

	// Add instructions sheet
	f.NewSheet("Instructions")
	instructions := [][]string{
		{"Property Data Import Instructions"},
		{""},
		{"1. Sheet name must match property subdomain (case-insensitive)"},
		{"2. Required columns: Tower, Floor, Unit, Area, Type"},
		{"3. For parking slots: Leave Tower empty, use 'park slot' as Type"},
		{"4. Multiple rows can represent multiple owners/SPAs/tenants for the same unit"},
		{"5. Users with email will receive invitation codes valid for 24 hours"},
		{"6. Supported types: studio, 1bedroom, 2bedroom, 3bedroom, penthouse, park slot"},
		{""},
		{"Column Descriptions:"},
		{"Tower - Building/Tower name (e.g., T1, A)"},
		{"Floor - Floor number"},
		{"Unit - Unit number"},
		{"Area - Area in square meters"},
		{"Type - Unit type (studio, 1bedroom, 2bedroom, park slot, etc.)"},
		{"Owner Name - Full name of the owner"},
		{"Owner Contact - Phone number"},
		{"Owner Email - Email address (optional but recommended)"},
		{"SPA Name - Special Power of Attorney holder name (optional)"},
		{"SPA Contact - SPA phone number"},
		{"SPA Email - SPA email address"},
		{"Tenant Name - Tenant name (optional)"},
		{"Tenant Contact - Tenant phone number"},
		{"Tenant Email - Tenant email address"},
		{"Move in - Tenant move-in date (format: YYYY/MM/DD)"},
		{"move out - Tenant move-out date (format: YYYY/MM/DD)"},
	}

	for i, row := range instructions {
		if len(row) > 0 {
			cell := fmt.Sprintf("A%d", i+1)
			f.SetCellValue("Instructions", cell, row[0])
		}
	}
	f.SetColWidth("Instructions", "A", "A", 80)

	// Write to buffer
	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, fmt.Errorf("failed to write excel to buffer: %w", err)
	}

	return buf.Bytes(), nil
}

// ParkingImportResult represents the result of parking import
type ParkingImportResult struct {
	TotalRows           int      `json:"total_rows"`
	ParkingSlotsCreated int      `json:"parking_slots_created"`
	OwnersCreated       int      `json:"owners_created"`
	SPACreated          int      `json:"spa_created"`
	Errors              []string `json:"errors,omitempty"`
}

// ParkingExcelRow represents a single row from parking Excel file
type ParkingExcelRow struct {
	RowNum       int
	SlotNumber   string
	SlotType     string
	Level        string
	Zone         string
	Size         string
	MonthlyRate  float64
	OwnerName    string
	OwnerContact string
	OwnerEmail   string
	SPANAME      string
	SPAContact   string
	SPAEmail     string
}

// ImportParkingFromExcel imports parking slots from an Excel file
func (s *ImportService) ImportParkingFromExcel(
	ctx context.Context,
	propertyID uint,
	subdomain string,
	propertyDB *gorm.DB,
	file io.Reader,
) (*ParkingImportResult, error) {
	result := &ParkingImportResult{
		Errors: []string{},
	}

	// Open Excel file
	f, err := excelize.OpenReader(file)
	if err != nil {
		return nil, &ImportError{
			Code:    "EXCEL_OPEN_ERROR",
			Message: "Failed to open Excel file",
			Details: map[string]interface{}{"error": err.Error()},
		}
	}
	defer f.Close()

	// Get all sheet names
	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, &ImportError{
			Code:    "NO_SHEETS",
			Message: "Excel file has no sheets",
		}
	}

	// Find matching sheet (case-insensitive comparison)
	var targetSheet string
	subdomainLower := strings.ToLower(subdomain)
	for _, sheet := range sheets {
		sheetLower := strings.ToLower(sheet)
		if sheetLower == "parking" || sheetLower == subdomainLower || strings.Contains(sheetLower, "parking") {
			targetSheet = sheet
			break
		}
	}

	// If no matching sheet found, use first sheet
	if targetSheet == "" {
		targetSheet = sheets[0]
	}

	// Get all rows from the sheet
	rows, err := f.GetRows(targetSheet)
	if err != nil {
		return nil, &ImportError{
			Code:    "READ_SHEET_ERROR",
			Message: "Failed to read sheet data",
			Details: map[string]interface{}{"error": err.Error()},
		}
	}

	if len(rows) < 2 {
		return nil, &ImportError{
			Code:    "NO_DATA",
			Message: "Sheet has no data rows",
		}
	}

	// Parse header row
	headerRow := rows[0]
	colMap := s.parseParkingHeaders(headerRow)

	// Validate required columns
	requiredCols := []string{"slot_number"}
	for _, col := range requiredCols {
		if _, ok := colMap[col]; !ok {
			return nil, &ImportError{
				Code:    "MISSING_COLUMN",
				Message: fmt.Sprintf("Required column '%s' not found", col),
				Details: map[string]interface{}{"found_columns": headerRow},
			}
		}
	}

	// Process data rows
	result.TotalRows = len(rows) - 1

	for i := 1; i < len(rows); i++ {
		row := rows[i]
		rowNum := i + 1

		// Parse row
		parkingRow := s.parseParkingRow(row, colMap, rowNum)

		// Skip empty rows
		if parkingRow.SlotNumber == "" {
			continue
		}

		// Create or update parking slot
		err := s.processParkingRow(ctx, propertyID, propertyDB, parkingRow, result)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("Row %d: %s", rowNum, err.Error()))
		}
	}

	return result, nil
}

// parseParkingHeaders parses parking Excel header row
func (s *ImportService) parseParkingHeaders(headers []string) map[string]int {
	colMap := make(map[string]int)

	headerMappings := map[string][]string{
		"slot_number":   {"slot number", "slotnumber", "slot_number", "slot", "number", "parking number", "parking_number"},
		"slot_type":     {"slot type", "slottype", "slot_type", "type", "parking type", "parking_type"},
		"level":         {"level", "floor", "lvl"},
		"zone":          {"zone", "area", "section"},
		"size":          {"size", "vehicle size", "vehicle_size", "slot size", "slot_size"},
		"monthly_rate":  {"monthly rate", "monthlyrate", "monthly_rate", "rate", "rent", "monthly rent"},
		"owner_name":    {"owner name", "ownername", "owner_name", "owner"},
		"owner_contact": {"owner contact", "ownercontact", "owner_contact", "contact", "phone"},
		"owner_email":   {"owner email", "owneremail", "owner_email", "email"},
		"spa_name":      {"spa name", "spaname", "spa_name", "spa"},
		"spa_contact":   {"spa contact", "spacontact", "spa_contact", "spa phone", "spa_phone"},
		"spa_email":     {"spa email", "spaemail", "spa_email", "spa mail"},
	}

	for i, header := range headers {
		headerLower := strings.ToLower(strings.TrimSpace(header))
		for key, variants := range headerMappings {
			for _, variant := range variants {
				if headerLower == variant {
					colMap[key] = i
					break
				}
			}
		}
	}

	return colMap
}

// parseParkingRow parses a single parking data row
func (s *ImportService) parseParkingRow(row []string, colMap map[string]int, rowNum int) ParkingExcelRow {
	getValue := func(key string) string {
		if idx, ok := colMap[key]; ok && idx < len(row) {
			return strings.TrimSpace(row[idx])
		}
		return ""
	}

	getFloat := func(key string) float64 {
		val := getValue(key)
		if val == "" {
			return 0
		}
		f, _ := strconv.ParseFloat(val, 64)
		return f
	}

	return ParkingExcelRow{
		RowNum:       rowNum,
		SlotNumber:   getValue("slot_number"),
		SlotType:     getValue("slot_type"),
		Level:        getValue("level"),
		Zone:         getValue("zone"),
		Size:         getValue("size"),
		MonthlyRate:  getFloat("monthly_rate"),
		OwnerName:    getValue("owner_name"),
		OwnerContact: getValue("owner_contact"),
		OwnerEmail:   getValue("owner_email"),
		SPANAME:      getValue("spa_name"),
		SPAContact:   getValue("spa_contact"),
		SPAEmail:     getValue("spa_email"),
	}
}

// processParkingRow processes a single parking row
func (s *ImportService) processParkingRow(ctx context.Context, propertyID uint, propertyDB *gorm.DB, row ParkingExcelRow, result *ParkingImportResult) error {
	// Normalize slot type
	slotType := s.normalizeParkingType(row.SlotType)

	// Normalize size
	size := s.normalizeParkingSize(row.Size)

	// Process owner first - OwnerID is required
	var ownerUserID uint
	if row.OwnerName != "" {
		ownerInfo := PersonInfo{
			Name:    row.OwnerName,
			Contact: row.OwnerContact,
			Email:   row.OwnerEmail,
		}

		userID, isNew, err := s.findOrCreateUser(ctx, propertyID, ownerInfo, "landlord")
		if err != nil {
			return fmt.Errorf("failed to create owner: %w", err)
		}

		if isNew {
			result.OwnersCreated++
		}
		ownerUserID = userID
	}

	// OwnerID is required - if not provided, return error
	if ownerUserID == 0 {
		return fmt.Errorf("parking slot %s requires an owner (Owner Name column)", row.SlotNumber)
	}

	// Check if slot already exists
	var existingSlot property.ParkingSlot
	err := propertyDB.Where("slot_number = ?", row.SlotNumber).First(&existingSlot).Error

	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("failed to check existing slot: %w", err)
	}

	var slotID uint
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// Determine vehicle type based on size
		vehicleType := "car"
		if size == "motorcycle" {
			vehicleType = "motorcycle"
		}

		// Create new parking slot with OwnerID
		newSlot := property.ParkingSlot{
			SlotNumber:         row.SlotNumber,
			OwnerID:            ownerUserID,
			ParkingType:        slotType,
			ParkingLevel:       strPtrImport(row.Level),
			ParkingZone:        strPtrImport(row.Zone),
			Size:               strPtrImport(size),
			VehicleTypeAllowed: vehicleType,
			Status:             "available",
		}

		if row.MonthlyRate > 0 {
			rateStr := fmt.Sprintf("%.2f", row.MonthlyRate)
			newSlot.MonthlyRent = &rateStr
		}

		if err := propertyDB.Create(&newSlot).Error; err != nil {
			return fmt.Errorf("failed to create parking slot: %w", err)
		}
		slotID = newSlot.ID
		result.ParkingSlotsCreated++
	} else {
		slotID = existingSlot.ID
		// Update OwnerID if slot exists but owner changed
		if existingSlot.OwnerID != ownerUserID {
			if err := propertyDB.Model(&existingSlot).Update("owner_id", ownerUserID).Error; err != nil {
				return fmt.Errorf("failed to update parking slot owner: %w", err)
			}
		}
	}

	// Create landlord parking slot association for ownership tracking
	err = s.createLandlordParkingSlot(propertyDB, slotID, ownerUserID)
	if err != nil {
		return fmt.Errorf("failed to associate owner with parking slot: %w", err)
	}

	// Process SPA information if provided
	if row.SPANAME != "" {
		spaInfo := PersonInfo{
			Name:    row.SPANAME,
			Contact: row.SPAContact,
			Email:   row.SPAEmail,
		}

		spaUserID, isNew, err := s.findOrCreateUser(ctx, propertyID, spaInfo, "spa")
		if err != nil {
			return fmt.Errorf("failed to create SPA: %w", err)
		}

		if isNew {
			result.SPACreated++
		}

		// Create SPA parking slot association
		err = s.createSPAParkingSlot(propertyDB, slotID, spaUserID)
		if err != nil {
			return fmt.Errorf("failed to associate SPA with parking slot: %w", err)
		}
	}

	return nil
}

// normalizeParkingType normalizes parking slot type
func (s *ImportService) normalizeParkingType(input string) string {
	input = strings.ToLower(strings.TrimSpace(input))

	typeMap := map[string]string{
		"indoor":      "indoor",
		"outdoor":     "outdoor",
		"covered":     "covered",
		"uncovered":   "uncovered",
		"basement":    "indoor",
		"mechanical":  "indoor",
		"open":        "outdoor",
		"underground": "indoor",
	}

	if normalized, ok := typeMap[input]; ok {
		return normalized
	}

	if input == "" {
		return "indoor" // Default
	}
	return input
}

// normalizeParkingSize normalizes parking slot size
func (s *ImportService) normalizeParkingSize(input string) string {
	input = strings.ToLower(strings.TrimSpace(input))

	sizeMap := map[string]string{
		"standard":   "standard",
		"compact":    "compact",
		"large":      "large",
		"motorcycle": "motorcycle",
		"small":      "compact",
		"big":        "large",
		"bike":       "motorcycle",
		"motor":      "motorcycle",
	}

	if normalized, ok := sizeMap[input]; ok {
		return normalized
	}

	if input == "" {
		return "standard" // Default
	}
	return input
}

// createLandlordParkingSlot creates landlord-parking slot association
func (s *ImportService) createLandlordParkingSlot(propertyDB *gorm.DB, slotID, userID uint) error {
	var existing property.LandlordParkingSlot
	err := propertyDB.Where("parking_slot_id = ? AND user_id = ?", slotID, userID).First(&existing).Error
	if err == nil {
		return nil // Already exists
	}

	now := time.Now()
	ownership := property.LandlordParkingSlot{
		UserID:              userID,
		ParkingSlotID:       slotID,
		OwnershipType:       "full",
		OwnershipPercentage: strPtrImport("100.00"),
		OwnershipStartDate:  &now,
	}

	return propertyDB.Create(&ownership).Error
}

// createSPAParkingSlot creates SPA-parking slot association
func (s *ImportService) createSPAParkingSlot(propertyDB *gorm.DB, slotID, spaUserID uint) error {
	var existing property.SPAParkingSlot
	err := propertyDB.Where("parking_slot_id = ? AND spa_user_id = ?", slotID, spaUserID).First(&existing).Error
	if err == nil {
		return nil // Already exists
	}

	now := time.Now()
	spaAssociation := property.SPAParkingSlot{
		SpaUserID:     spaUserID,
		ParkingSlotID: slotID,
		Scope:         "full",
		IsActive:      true,
		CreatedAt:     now,
	}

	return propertyDB.Create(&spaAssociation).Error
}

// GenerateParkingTemplate generates an Excel template for parking import
func (s *ImportService) GenerateParkingTemplate(propertyName string) ([]byte, error) {
	f := excelize.NewFile()

	// Create parking sheet
	sheetName := "{subdomain}"
	f.SetSheetName("Sheet1", sheetName)

	// Set headers
	headers := []string{
		"Slot Number", "Slot Type", "Level", "Zone", "Size", "Monthly Rate",
		"Owner Name", "Owner Contact", "Owner Email",
		"SPA Name", "SPA Contact", "SPA Email",
	}

	for i, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheetName, cell, header)
	}

	// Style headers
	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"10B981"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	f.SetCellStyle(sheetName, "A1", "L1", headerStyle)

	// Add sample data
	sampleData := [][]interface{}{
		{"P-A01", "indoor", "B1", "Zone A", "standard", 2000, "John Doe", "09123456789", "john@example.com", "ABC Realty", "09112233445", "spa@abcrealty.com"},
		{"P-B02", "outdoor", "G", "Zone B", "compact", 1500, "Jane Smith", "09187654321", "jane@example.com", "", "", ""},
		{"P-M01", "covered", "B2", "Zone C", "motorcycle", 500, "", "", "", "XYZ Properties", "09115556677", "contact@xyzproperties.com"},
	}

	for i, row := range sampleData {
		for j, val := range row {
			cell, _ := excelize.CoordinatesToCellName(j+1, i+2)
			f.SetCellValue(sheetName, cell, val)
		}
	}

	// Set column widths
	f.SetColWidth(sheetName, "A", "A", 15)
	f.SetColWidth(sheetName, "B", "B", 12)
	f.SetColWidth(sheetName, "C", "C", 10)
	f.SetColWidth(sheetName, "D", "D", 12)
	f.SetColWidth(sheetName, "E", "E", 12)
	f.SetColWidth(sheetName, "F", "F", 15)
	f.SetColWidth(sheetName, "G", "G", 20)
	f.SetColWidth(sheetName, "H", "H", 15)
	f.SetColWidth(sheetName, "I", "I", 25)

	// Add instructions sheet
	f.NewSheet("Instructions")
	instructions := [][]string{
		{"Parking Import Instructions"},
		{""},
		{"1. Sheet name should match property subdomain"},
		{"2. Required columns: Slot Number"},
		{"3. Slot Type options: indoor, outdoor, covered, uncovered"},
		{"4. Size options: standard, compact, large, motorcycle"},
		{"5. Owner information is optional - leave blank if no owner assigned"},
		{"6. Owners with email will receive invitation codes valid for 24 hours"},
		{""},
		{"Column Descriptions:"},
		{"Slot Number - Unique parking slot identifier (e.g., P-A01, P-B02)"},
		{"Slot Type - Type of parking: indoor, outdoor, covered, uncovered"},
		{"Level - Floor/Level (e.g., B1, B2, G, L1, L2)"},
		{"Zone - Parking zone or area (e.g., Zone A, North, etc.)"},
		{"Size - Vehicle size: standard, compact, large, motorcycle"},
		{"Monthly Rate - Monthly rental rate (optional)"},
		{"Owner Name - Full name of the parking slot owner (optional)"},
		{"Owner Contact - Owner phone number (optional)"},
		{"Owner Email - Owner email address (optional)"},
	}

	for i, row := range instructions {
		if len(row) > 0 {
			cell := fmt.Sprintf("A%d", i+1)
			f.SetCellValue("Instructions", cell, row[0])
		}
	}
	f.SetColWidth("Instructions", "A", "A", 80)

	// Write to buffer
	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, fmt.Errorf("failed to write excel to buffer: %w", err)
	}

	return buf.Bytes(), nil
}
