package service

import (
	"errors"
	"fmt"
	"time"

	"homexai/internal/models/property"
	propertyRepo "homexai/internal/repository/property"

	"gorm.io/gorm"
)

type MarketplaceService struct {
	propertyDB *gorm.DB
}

func NewMarketplaceService(propertyDB *gorm.DB) *MarketplaceService {
	return &MarketplaceService{
		propertyDB: propertyDB,
	}
}

// ========== Service Listing Management ==========

// CreateServiceListing creates a new service listing
func (s *MarketplaceService) CreateServiceListing(listing *property.ServiceListing) error {
	repo := propertyRepo.NewServiceListingRepository(s.propertyDB)

	// Validate third party contact for third party services
	if !listing.IsPropertyService && (listing.ThirdPartyContact == nil || *listing.ThirdPartyContact == "") {
		return errors.New("third party contact is required for third party services")
	}

	// Set default status
	if listing.Status == "" {
		listing.Status = property.ServiceListingStatusActive
	}

	return repo.Create(listing)
}

// GetServiceListing gets a service listing by ID
func (s *MarketplaceService) GetServiceListing(id uint) (*property.ServiceListing, error) {
	repo := propertyRepo.NewServiceListingRepository(s.propertyDB)
	return repo.FindByID(id)
}

// UpdateServiceListing updates a service listing
func (s *MarketplaceService) UpdateServiceListing(listing *property.ServiceListing) error {
	repo := propertyRepo.NewServiceListingRepository(s.propertyDB)

	// Validate third party contact for third party services
	if !listing.IsPropertyService && (listing.ThirdPartyContact == nil || *listing.ThirdPartyContact == "") {
		return errors.New("third party contact is required for third party services")
	}

	return repo.Update(listing)
}

// DeleteServiceListing deletes a service listing
func (s *MarketplaceService) DeleteServiceListing(id uint) error {
	repo := propertyRepo.NewServiceListingRepository(s.propertyDB)

	// Check if there are any orders for this listing
	orderRepo := propertyRepo.NewServiceOrderRepository(s.propertyDB)
	orders, err := orderRepo.ListByServiceListing(id)
	if err != nil {
		return err
	}
	if len(orders) > 0 {
		return errors.New("cannot delete service listing with existing orders")
	}

	return repo.Delete(id)
}

// ListServiceListings lists service listings with filters
func (s *MarketplaceService) ListServiceListings(serviceType, status string, page, perPage int) ([]property.ServiceListing, int64, error) {
	repo := propertyRepo.NewServiceListingRepository(s.propertyDB)
	return repo.List(serviceType, status, page, perPage)
}

// ListActiveServiceListings lists active service listings
func (s *MarketplaceService) ListActiveServiceListings(serviceType string) ([]property.ServiceListing, error) {
	repo := propertyRepo.NewServiceListingRepository(s.propertyDB)
	return repo.ListActive(serviceType)
}

// UpdateServiceListingStatus updates service listing status (上架/下架)
func (s *MarketplaceService) UpdateServiceListingStatus(id uint, status string) error {
	repo := propertyRepo.NewServiceListingRepository(s.propertyDB)

	if status != property.ServiceListingStatusActive && status != property.ServiceListingStatusInactive {
		return errors.New("invalid status")
	}

	return repo.UpdateStatus(id, status)
}

// ========== Service Order Management ==========

// CreateServiceOrder creates a new service order
func (s *MarketplaceService) CreateServiceOrder(order *property.ServiceOrder) error {
	orderRepo := propertyRepo.NewServiceOrderRepository(s.propertyDB)
	listingRepo := propertyRepo.NewServiceListingRepository(s.propertyDB)

	// Verify service listing exists and is active
	listing, err := listingRepo.FindByID(order.ServiceListingID)
	if err != nil {
		return errors.New("service listing not found")
	}
	if listing.Status != property.ServiceListingStatusActive {
		return errors.New("service listing is not active")
	}

	// Generate order number
	if order.OrderNumber == "" {
		order.OrderNumber = s.generateOrderNumber()
	}

	// Set default status
	if order.Status == "" {
		order.Status = property.ServiceOrderStatusInService
	}

	return orderRepo.Create(order)
}

// GetServiceOrder gets a service order by ID
func (s *MarketplaceService) GetServiceOrder(id uint) (*property.ServiceOrder, error) {
	repo := propertyRepo.NewServiceOrderRepository(s.propertyDB)
	return repo.FindByID(id)
}

// ListServiceOrders lists service orders with filters
func (s *MarketplaceService) ListServiceOrders(userID uint, status string, page, perPage int) ([]property.ServiceOrder, int64, error) {
	repo := propertyRepo.NewServiceOrderRepository(s.propertyDB)
	return repo.List(userID, status, page, perPage)
}

// ListUserServiceOrders lists service orders for a user
func (s *MarketplaceService) ListUserServiceOrders(userID uint, page, perPage int) ([]property.ServiceOrder, int64, error) {
	repo := propertyRepo.NewServiceOrderRepository(s.propertyDB)
	return repo.List(userID, "", page, perPage)
}

// AssignStaff assigns staff to a service order
func (s *MarketplaceService) AssignStaff(orderID, staffID uint) error {
	repo := propertyRepo.NewServiceOrderRepository(s.propertyDB)

	// Verify order exists
	order, err := repo.FindByID(orderID)
	if err != nil {
		return errors.New("order not found")
	}
	if order.Status != property.ServiceOrderStatusInService {
		return errors.New("can only assign staff to orders in service")
	}

	return repo.AssignStaff(orderID, staffID)
}

// CompleteServiceOrder marks a service order as completed by staff
func (s *MarketplaceService) CompleteServiceOrder(orderID uint) error {
	orderRepo := propertyRepo.NewServiceOrderRepository(s.propertyDB)

	// Get order
	order, err := orderRepo.FindByID(orderID)
	if err != nil {
		return errors.New("order not found")
	}
	if order.Status != property.ServiceOrderStatusInService {
		return errors.New("order is not in service")
	}

	// If it's a property service, add service fee item to existing bill or create new one
	// Note: Service fees should be added to the unit's monthly bill as a bill item
	// This will be handled when accountant creates the monthly bill
	// For now, we just mark the order as completed

	// Mark order as completed
	err = orderRepo.CompleteOrder(orderID)
	if err != nil {
		return err
	}

	return nil
}

// ConfirmServiceOrder confirms a service order by user
func (s *MarketplaceService) ConfirmServiceOrder(orderID, userID uint) error {
	repo := propertyRepo.NewServiceOrderRepository(s.propertyDB)

	// Verify order exists and belongs to user
	order, err := repo.FindByID(orderID)
	if err != nil {
		return errors.New("order not found")
	}
	if order.UserID != userID {
		return errors.New("unauthorized")
	}
	if order.Status != property.ServiceOrderStatusCompleted {
		return errors.New("order is not completed")
	}

	return repo.ConfirmOrder(orderID)
}

// CancelServiceOrder cancels a service order
func (s *MarketplaceService) CancelServiceOrder(orderID, userID uint) error {
	repo := propertyRepo.NewServiceOrderRepository(s.propertyDB)

	// Verify order exists and belongs to user
	order, err := repo.FindByID(orderID)
	if err != nil {
		return errors.New("order not found")
	}
	if order.UserID != userID {
		return errors.New("unauthorized")
	}
	if order.Status == property.ServiceOrderStatusCompleted {
		return errors.New("cannot cancel completed order")
	}
	if order.Status == property.ServiceOrderStatusCancelled {
		return errors.New("order already cancelled")
	}

	return repo.CancelOrder(orderID)
}

// Helper functions
func (s *MarketplaceService) generateOrderNumber() string {
	now := time.Now()
	return fmt.Sprintf("SO-%s-%d", now.Format("20060102"), now.Unix())
}
