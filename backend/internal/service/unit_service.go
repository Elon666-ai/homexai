package service

import (
	"errors"

	"homexai/internal/models/property"
	propertyRepo "homexai/internal/repository/property"

	"gorm.io/gorm"
)

type UnitService struct {
	propertyDB *gorm.DB
}

func NewUnitService(propertyDB *gorm.DB) *UnitService {
	return &UnitService{
		propertyDB: propertyDB,
	}
}

// CreateUnit creates a new unit
func (s *UnitService) CreateUnit(unit *property.Unit) error {
	repo := propertyRepo.NewUnitRepository(s.propertyDB)

	// Check if unit number already exists
	exists, err := repo.ExistsByNumber(unit.UnitNumber, unit.UnitType)
	if err != nil {
		return err
	}
	if exists {
		return errors.New("unit number already exists for this type")
	}

	return repo.Create(unit)
}

// GetUnit gets unit by ID
func (s *UnitService) GetUnit(id uint) (*property.Unit, error) {
	repo := propertyRepo.NewUnitRepository(s.propertyDB)
	return repo.FindByID(id)
}

// UpdateUnit updates a unit
func (s *UnitService) UpdateUnit(id uint, updates map[string]interface{}) error {
	repo := propertyRepo.NewUnitRepository(s.propertyDB)

	// Check if unit exists
	_, err := repo.FindByID(id)
	if err != nil {
		return err
	}

	return repo.UpdateFields(id, updates)
}

// DeleteUnit deletes a unit
func (s *UnitService) DeleteUnit(id uint) error {
	repo := propertyRepo.NewUnitRepository(s.propertyDB)
	return repo.Delete(id)
}

// ListUnits lists units with filters and pagination
func (s *UnitService) ListUnits(unitType, status string, page, perPage int) ([]property.Unit, int64, error) {
	repo := propertyRepo.NewUnitRepository(s.propertyDB)
	return repo.List(unitType, status, page, perPage)
}

// ListUnitsWithExclude lists units with filters, pagination, and type exclusion
func (s *UnitService) ListUnitsWithExclude(unitType, excludeType, status string, page, perPage int) ([]property.Unit, int64, error) {
	repo := propertyRepo.NewUnitRepository(s.propertyDB)
	return repo.ListWithExclude(unitType, excludeType, status, page, perPage)
}

// ListUnitsWithFilters lists units with comprehensive filters including unit number
func (s *UnitService) ListUnitsWithFilters(unitNumber, unitType, excludeType, status string, page, perPage int) ([]property.Unit, int64, error) {
	repo := propertyRepo.NewUnitRepository(s.propertyDB)
	return repo.ListWithFilters(unitNumber, unitType, excludeType, status, page, perPage)
}

// ListApartments lists all apartments
func (s *UnitService) ListApartments() ([]property.Unit, error) {
	repo := propertyRepo.NewUnitRepository(s.propertyDB)
	return repo.ListByType(property.UnitTypeApartment)
}

// ListStorageUnits lists all storage units
func (s *UnitService) ListStorageUnits() ([]property.Unit, error) {
	repo := propertyRepo.NewUnitRepository(s.propertyDB)
	return repo.ListByType(property.UnitTypeStorage)
}

// ListCommercialUnits lists all commercial units
func (s *UnitService) ListCommercialUnits() ([]property.Unit, error) {
	repo := propertyRepo.NewUnitRepository(s.propertyDB)
	return repo.ListByType(property.UnitTypeCommercial)
}

// ListAvailableUnits lists available units
func (s *UnitService) ListAvailableUnits(unitType string) ([]property.Unit, error) {
	repo := propertyRepo.NewUnitRepository(s.propertyDB)
	return repo.ListAvailable(unitType)
}

// SearchUnits searches units by query
func (s *UnitService) SearchUnits(query, unitType, status string, page, perPage int) ([]property.Unit, int64, error) {
	repo := propertyRepo.NewUnitRepository(s.propertyDB)
	return repo.Search(query, unitType, status, page, perPage)
}

// MarkAsOccupied marks unit as occupied
func (s *UnitService) MarkAsOccupied(id uint) error {
	repo := propertyRepo.NewUnitRepository(s.propertyDB)
	return repo.UpdateStatus(id, property.UnitStatusOccupied)
}

// MarkAsAvailable marks unit as available
func (s *UnitService) MarkAsAvailable(id uint) error {
	repo := propertyRepo.NewUnitRepository(s.propertyDB)
	return repo.UpdateStatus(id, property.UnitStatusAvailable)
}

// MarkAsMaintenance marks unit as under maintenance
func (s *UnitService) MarkAsMaintenance(id uint) error {
	repo := propertyRepo.NewUnitRepository(s.propertyDB)
	return repo.UpdateStatus(id, property.UnitStatusMaintenance)
}

// GetStatistics gets unit statistics (不包含停车位，停车位已独立到 ParkingSlotService)
func (s *UnitService) GetStatistics() (map[string]interface{}, error) {
	repo := propertyRepo.NewUnitRepository(s.propertyDB)

	totalApartments, err := repo.CountByType(property.UnitTypeApartment)
	if err != nil {
		return nil, err
	}

	totalStorage, err := repo.CountByType(property.UnitTypeStorage)
	if err != nil {
		return nil, err
	}

	totalCommercial, err := repo.CountByType(property.UnitTypeCommercial)
	if err != nil {
		return nil, err
	}

	availableUnits, err := repo.CountByStatus(property.UnitStatusAvailable)
	if err != nil {
		return nil, err
	}

	occupiedUnits, err := repo.CountByStatus(property.UnitStatusOccupied)
	if err != nil {
		return nil, err
	}

	maintenanceUnits, err := repo.CountByStatus(property.UnitStatusMaintenance)
	if err != nil {
		return nil, err
	}

	totalUnits := totalApartments + totalStorage + totalCommercial
	occupancyRate := float64(0)
	if totalUnits > 0 {
		occupancyRate = float64(occupiedUnits) / float64(totalUnits) * 100
	}

	return map[string]interface{}{
		"total_apartments":  totalApartments,
		"total_storage":     totalStorage,
		"total_commercial":  totalCommercial,
		"total_units":       totalUnits,
		"available_units":   availableUnits,
		"occupied_units":    occupiedUnits,
		"maintenance_units": maintenanceUnits,
		"occupancy_rate":    occupancyRate,
	}, nil
}
