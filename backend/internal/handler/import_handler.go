package handler

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"homexai/internal/middleware"
	"homexai/internal/models/master"
	"homexai/internal/service"
	"homexai/internal/utils"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
)

// ImportHandler handles import-related requests
type ImportHandler struct {
	masterDB      *gorm.DB
	importService *service.ImportService
}

// NewImportHandler creates a new import handler
func NewImportHandler(masterDB *gorm.DB, importService *service.ImportService) *ImportHandler {
	return &ImportHandler{
		masterDB:      masterDB,
		importService: importService,
	}
}

// ImportUnits handles Excel file upload and async import
// @Summary Import units from Excel file
// @Description Property Admin imports units, owners, SPAs, and tenants from Excel file (async)
// @Tags Import
// @Accept multipart/form-data
// @Produce json
// @Param file formdata file true "Excel file"
// @Success 200 {object} utils.Response
// @Failure 400 {object} utils.Response
// @Failure 401 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /api/v1/property-admin/import/units [post]
func (h *ImportHandler) ImportUnits(c *gin.Context) {
	// Get property ID from context
	propertyID := middleware.GetPropertyID(c)
	if propertyID == 0 {
		utils.Error(c, http.StatusBadRequest, "Property ID not found")
		return
	}

	// Get subdomain from context
	subdomain, exists := c.Get("subdomain")
	if !exists {
		utils.Error(c, http.StatusBadRequest, "Subdomain not found")
		return
	}

	// Get user ID
	userID := middleware.GetUserID(c)

	// Get property database connection
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		utils.Error(c, http.StatusInternalServerError, "Property database not found")
		return
	}

	// Get uploaded file
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "Failed to get uploaded file: "+err.Error())
		return
	}
	defer file.Close()

	// Validate file extension
	ext := filepath.Ext(header.Filename)
	if ext != ".xlsx" && ext != ".xls" {
		utils.Error(c, http.StatusBadRequest, "Invalid file format. Only .xlsx and .xls files are allowed")
		return
	}

	// Validate file type (strict mode: check if file matches expected type)
	// Read file into memory for validation
	fileBytes, err := io.ReadAll(file)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "Failed to read file: "+err.Error())
		return
	}
	file.Close() // Close the original file

	// Reopen file for excelize
	excelFile, err := excelize.OpenReader(bytes.NewReader(fileBytes))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "Failed to open Excel file: "+err.Error())
		return
	}
	defer excelFile.Close()

	// Validate file type
	detectedType, isValid, err := h.importService.ValidateFileType(excelFile, "units")
	if err != nil {
		if importErr, ok := err.(*service.ImportError); ok && importErr.Code == "FILE_TYPE_MISMATCH" {
			utils.Error(c, http.StatusBadRequest, importErr.Message)
			return
		}
		// For other errors, log but continue (non-critical)
		fmt.Printf("Warning: File type validation error: %v\n", err)
	} else if !isValid && detectedType == "parking" {
		utils.Error(c, http.StatusBadRequest, "This file appears to be a Parking import file. Please use the Parking import page.")
		return
	}

	// Create upload directory
	uploadDir := fmt.Sprintf("./uploads/imports/%d", propertyID)
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		utils.Error(c, http.StatusInternalServerError, "Failed to create upload directory")
		return
	}

	// Generate unique filename
	filename := fmt.Sprintf("units_%d_%d%s", time.Now().UnixNano(), propertyID, ext)
	filePath := filepath.Join(uploadDir, filename)

	// Save file to disk using bytes from memory
	dst, err := os.Create(filePath)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "Failed to save file")
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, bytes.NewReader(fileBytes)); err != nil {
		utils.Error(c, http.StatusInternalServerError, "Failed to save file")
		return
	}

	// Create import task record with "uploaded" status
	task := master.ImportTask{
		PropertyID:  propertyID,
		Subdomain:   subdomain.(string),
		FileName:    header.Filename,
		FilePath:    filePath,
		TaskType:    master.ImportTaskTypeUnit,
		TotalRows:   0,
		SuccessRows: 0,
		FailedRows:  0,
		Status:      master.ImportTaskStatusUploaded, // File uploaded, waiting for import
		CreatedBy:   userID,
	}

	if err := h.masterDB.Create(&task).Error; err != nil {
		utils.Error(c, http.StatusInternalServerError, "Failed to create import task")
		return
	}

	// Return with task info - import will be triggered separately
	utils.Success(c, "File uploaded successfully. Ready for import.", gin.H{
		"task_id":   task.ID,
		"status":    task.Status,
		"file_name": header.Filename,
		"message":   "File uploaded successfully. Please click 'Import' to start processing.",
	})
}

// processUnitsImportAsync processes Excel file in background
func (h *ImportHandler) processUnitsImportAsync(taskID uint, propertyID uint, subdomain string, propertyDB *gorm.DB, filePath string) {
	// Update task status to processing
	h.masterDB.Model(&master.ImportTask{}).Where("id = ?", taskID).Update("status", master.ImportTaskStatusProcessing)

	// Open file
	file, err := os.Open(filePath)
	if err != nil {
		h.failTask(taskID, fmt.Sprintf("Failed to open file: %v", err))
		return
	}
	defer file.Close()

	// Process Excel with background context
	ctx := context.Background()
	result, err := h.importService.ImportUnitsFromExcel(
		ctx,
		propertyID,
		subdomain,
		propertyDB,
		file,
	)

	if err != nil {
		errMsg := err.Error()
		if importErr, ok := err.(*service.ImportError); ok {
			errMsg = fmt.Sprintf("%s: %v", importErr.Message, importErr.Details)
		}
		h.failTask(taskID, errMsg)
		return
	}

	// Update task with results
	now := time.Now()
	var errorLog *string
	if len(result.Errors) > 0 {
		errStr := fmt.Sprintf("%v", result.Errors)
		errorLog = &errStr
	}

	h.masterDB.Model(&master.ImportTask{}).Where("id = ?", taskID).Updates(map[string]interface{}{
		"status":       master.ImportTaskStatusCompleted,
		"total_rows":   result.TotalRows,
		"success_rows": result.ProcessedRows - result.FailedRows,
		"failed_rows":  result.FailedRows,
		"error_log":    errorLog,
		"completed_at": &now,
	})

	fmt.Printf("📦 Import task %d completed: %d rows processed, %d failed\n", taskID, result.ProcessedRows, result.FailedRows)
}

// StartImport starts the import process for an uploaded file
// @Summary Start import process
// @Description Property Admin starts the import process for an uploaded file
// @Tags Import
// @Produce json
// @Param id path int true "Task ID"
// @Success 200 {object} utils.Response
// @Failure 400 {object} utils.Response
// @Failure 401 {object} utils.Response
// @Failure 404 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /api/v1/property-admin/import/units/{id}/start [post]
func (h *ImportHandler) StartImport(c *gin.Context) {
	// Get property ID from context
	propertyID := middleware.GetPropertyID(c)
	if propertyID == 0 {
		utils.Error(c, http.StatusBadRequest, "Property ID not found")
		return
	}

	// Get subdomain from context
	subdomain, exists := c.Get("subdomain")
	if !exists {
		utils.Error(c, http.StatusBadRequest, "Subdomain not found")
		return
	}

	// Get property database connection
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		utils.Error(c, http.StatusInternalServerError, "Property database not found")
		return
	}

	// Get task ID from URL
	taskID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "Invalid task ID")
		return
	}

	// Get task from database
	var task master.ImportTask
	if err := h.masterDB.Where("id = ? AND property_id = ?", taskID, propertyID).First(&task).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			utils.Error(c, http.StatusNotFound, "Import task not found")
			return
		}
		utils.Error(c, http.StatusInternalServerError, "Failed to fetch import task")
		return
	}

	// Check if task is in "uploaded" status
	if task.Status != master.ImportTaskStatusUploaded {
		utils.Error(c, http.StatusBadRequest, fmt.Sprintf("Task is not ready for import. Current status: %s", task.Status))
		return
	}

	// Check task type and start appropriate import process
	if task.TaskType == master.ImportTaskTypeUnit {
		// Start async processing for units
		go h.processUnitsImportAsync(task.ID, propertyID, subdomain.(string), propertyDB, task.FilePath)
	} else if task.TaskType == master.ImportTaskTypeParking {
		// Start async processing for parking
		go h.processParkingImportAsync(task.ID, propertyID, subdomain.(string), propertyDB, task.FilePath)
	} else {
		utils.Error(c, http.StatusBadRequest, fmt.Sprintf("Unsupported task type: %s", task.TaskType))
		return
	}

	// Return immediately with task info
	utils.Success(c, "Import process started successfully.", gin.H{
		"task_id":   task.ID,
		"status":    master.ImportTaskStatusProcessing,
		"file_name": task.FileName,
		"message":   "Import is being processed in the background.",
	})
}

// failTask marks a task as failed
func (h *ImportHandler) failTask(taskID uint, errMsg string) {
	now := time.Now()
	h.masterDB.Model(&master.ImportTask{}).Where("id = ?", taskID).Updates(map[string]interface{}{
		"status":       master.ImportTaskStatusFailed,
		"error_log":    &errMsg,
		"completed_at": &now,
	})
	fmt.Printf("❌ Import task %d failed: %s\n", taskID, errMsg)
}

// ImportParking handles Excel file upload and async parking import
// @Summary Import parking slots from Excel file
// @Description Property Admin imports parking slots and owners from Excel file (async)
// @Tags Import
// @Accept multipart/form-data
// @Produce json
// @Param file formdata file true "Excel file"
// @Success 200 {object} utils.Response
// @Failure 400 {object} utils.Response
// @Failure 401 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /api/v1/property-admin/import/parking [post]
func (h *ImportHandler) ImportParking(c *gin.Context) {
	// Get property ID from context
	propertyID := middleware.GetPropertyID(c)
	if propertyID == 0 {
		utils.Error(c, http.StatusBadRequest, "Property ID not found")
		return
	}

	// Get subdomain from context
	subdomain, exists := c.Get("subdomain")
	if !exists {
		utils.Error(c, http.StatusBadRequest, "Subdomain not found")
		return
	}

	// Get user ID
	userID := middleware.GetUserID(c)

	// Get property database connection
	propertyDB := middleware.GetPropertyDB(c)
	if propertyDB == nil {
		utils.Error(c, http.StatusInternalServerError, "Property database not found")
		return
	}

	// Get uploaded file
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "Failed to get uploaded file: "+err.Error())
		return
	}
	defer file.Close()

	// Validate file extension
	ext := filepath.Ext(header.Filename)
	if ext != ".xlsx" && ext != ".xls" {
		utils.Error(c, http.StatusBadRequest, "Invalid file format. Only .xlsx and .xls files are allowed")
		return
	}

	// Validate file type (strict mode: check if file matches expected type)
	// Read file into memory for validation
	fileBytes, err := io.ReadAll(file)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "Failed to read file: "+err.Error())
		return
	}
	file.Close() // Close the original file

	// Reopen file for excelize
	excelFile, err := excelize.OpenReader(bytes.NewReader(fileBytes))
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "Failed to open Excel file: "+err.Error())
		return
	}
	defer excelFile.Close()

	// Validate file type
	detectedType, isValid, err := h.importService.ValidateFileType(excelFile, "parking")
	if err != nil {
		if importErr, ok := err.(*service.ImportError); ok && importErr.Code == "FILE_TYPE_MISMATCH" {
			utils.Error(c, http.StatusBadRequest, importErr.Message)
			return
		}
		// For other errors, log but continue (non-critical)
		fmt.Printf("Warning: File type validation error: %v\n", err)
	} else if !isValid && detectedType == "units" {
		utils.Error(c, http.StatusBadRequest, "This file appears to be a Units import file. Please use the Units import page.")
		return
	}

	// Create upload directory
	uploadDir := fmt.Sprintf("./uploads/imports/%d", propertyID)
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		utils.Error(c, http.StatusInternalServerError, "Failed to create upload directory")
		return
	}

	// Generate unique filename
	filename := fmt.Sprintf("parking_%d_%d%s", time.Now().UnixNano(), propertyID, ext)
	filePath := filepath.Join(uploadDir, filename)

	// Save file to disk using bytes from memory
	dst, err := os.Create(filePath)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "Failed to save file")
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, bytes.NewReader(fileBytes)); err != nil {
		utils.Error(c, http.StatusInternalServerError, "Failed to save file")
		return
	}

	// Create import task record with "uploaded" status
	task := master.ImportTask{
		PropertyID:  propertyID,
		Subdomain:   subdomain.(string),
		FileName:    header.Filename,
		FilePath:    filePath,
		TaskType:    master.ImportTaskTypeParking,
		TotalRows:   0,
		SuccessRows: 0,
		FailedRows:  0,
		Status:      master.ImportTaskStatusUploaded, // File uploaded, waiting for import
		CreatedBy:   userID,
	}

	if err := h.masterDB.Create(&task).Error; err != nil {
		utils.Error(c, http.StatusInternalServerError, "Failed to create import task")
		return
	}

	// Return with task info - import will be triggered separately
	utils.Success(c, "File uploaded successfully. Ready for import.", gin.H{
		"task_id":   task.ID,
		"status":    task.Status,
		"file_name": header.Filename,
		"message":   "File uploaded successfully. Please click 'Import' to start processing.",
	})
}

// processParkingImportAsync processes parking Excel file in background
func (h *ImportHandler) processParkingImportAsync(taskID uint, propertyID uint, subdomain string, propertyDB *gorm.DB, filePath string) {
	// Update task status to processing
	h.masterDB.Model(&master.ImportTask{}).Where("id = ?", taskID).Update("status", master.ImportTaskStatusProcessing)

	// Open file
	file, err := os.Open(filePath)
	if err != nil {
		h.failTask(taskID, fmt.Sprintf("Failed to open file: %v", err))
		return
	}
	defer file.Close()

	// Process Excel with background context
	ctx := context.Background()
	result, err := h.importService.ImportParkingFromExcel(
		ctx,
		propertyID,
		subdomain,
		propertyDB,
		file,
	)

	if err != nil {
		errMsg := err.Error()
		if importErr, ok := err.(*service.ImportError); ok {
			errMsg = fmt.Sprintf("%s: %v", importErr.Message, importErr.Details)
		}
		h.failTask(taskID, errMsg)
		return
	}

	// Update task with results
	now := time.Now()
	var errorLog *string
	if len(result.Errors) > 0 {
		errStr := fmt.Sprintf("%v", result.Errors)
		errorLog = &errStr
	}

	failedRows := len(result.Errors)
	successRows := result.ParkingSlotsCreated

	h.masterDB.Model(&master.ImportTask{}).Where("id = ?", taskID).Updates(map[string]interface{}{
		"status":       master.ImportTaskStatusCompleted,
		"total_rows":   result.TotalRows,
		"success_rows": successRows,
		"failed_rows":  failedRows,
		"error_log":    errorLog,
		"completed_at": &now,
	})

	fmt.Printf("📦 Parking import task %d completed: %d slots created, %d failed\n", taskID, successRows, failedRows)
}

// GetImportTasks returns list of import tasks for the property
// @Summary Get import tasks
// @Description Get list of import tasks for the current property
// @Tags Import
// @Produce json
// @Success 200 {object} utils.Response
// @Failure 401 {object} utils.Response
// @Failure 500 {object} utils.Response
// @Router /api/v1/property-admin/import/tasks [get]
func (h *ImportHandler) GetImportTasks(c *gin.Context) {
	propertyID := middleware.GetPropertyID(c)
	if propertyID == 0 {
		utils.Error(c, http.StatusBadRequest, "Property ID not found")
		return
	}

	var tasks []master.ImportTask
	if err := h.masterDB.Where("property_id = ?", propertyID).
		Order("created_at DESC").
		Limit(50).
		Find(&tasks).Error; err != nil {
		utils.Error(c, http.StatusInternalServerError, "Failed to fetch import tasks")
		return
	}

	utils.Success(c, "Import tasks retrieved", tasks)
}

// GetImportTask returns a specific import task
// @Summary Get import task by ID
// @Description Get details of a specific import task
// @Tags Import
// @Produce json
// @Param id path int true "Task ID"
// @Success 200 {object} utils.Response
// @Failure 401 {object} utils.Response
// @Failure 404 {object} utils.Response
// @Router /api/v1/property-admin/import/tasks/{id} [get]
func (h *ImportHandler) GetImportTask(c *gin.Context) {
	propertyID := middleware.GetPropertyID(c)
	if propertyID == 0 {
		utils.Error(c, http.StatusBadRequest, "Property ID not found")
		return
	}

	taskID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "Invalid task ID")
		return
	}

	var task master.ImportTask
	if err := h.masterDB.Where("id = ? AND property_id = ?", taskID, propertyID).First(&task).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			utils.Error(c, http.StatusNotFound, "Import task not found")
			return
		}
		utils.Error(c, http.StatusInternalServerError, "Failed to fetch import task")
		return
	}

	utils.Success(c, "Import task retrieved", task)
}

// DownloadTemplate generates and downloads an Excel template
// @Summary Download import template
// @Description Download an Excel template for property data import
// @Tags Import
// @Produce application/vnd.openxmlformats-officedocument.spreadsheetml.sheet
// @Success 200 {file} binary
// @Failure 500 {object} utils.Response
// @Router /api/v1/property-admin/import/template [get]
func (h *ImportHandler) DownloadTemplate(c *gin.Context) {
	// Get property name from context
	propertyName, exists := c.Get("property_name")
	if !exists {
		propertyName = "Property"
	}

	// Generate template
	templateData, err := h.importService.GenerateTemplate(propertyName.(string))
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "Failed to generate template: "+err.Error())
		return
	}

	// Set response headers
	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Disposition", "attachment; filename=property_import_template.xlsx")
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Transfer-Encoding", "binary")
	c.Header("Expires", "0")
	c.Header("Cache-Control", "must-revalidate")
	c.Header("Pragma", "public")

	c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", templateData)
}

// DownloadParkingTemplate generates and downloads an Excel template for parking import
// @Summary Download parking import template
// @Description Download an Excel template for parking slot import
// @Tags Import
// @Produce application/vnd.openxmlformats-officedocument.spreadsheetml.sheet
// @Success 200 {file} binary
// @Failure 500 {object} utils.Response
// @Router /api/v1/property-admin/import/parking/template [get]
func (h *ImportHandler) DownloadParkingTemplate(c *gin.Context) {
	// Get property name from context
	propertyName, exists := c.Get("property_name")
	if !exists {
		propertyName = "Property"
	}

	// Generate template
	templateData, err := h.importService.GenerateParkingTemplate(propertyName.(string))
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "Failed to generate template: "+err.Error())
		return
	}

	// Set response headers
	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Disposition", "attachment; filename=parking_import_template.xlsx")
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Transfer-Encoding", "binary")
	c.Header("Expires", "0")
	c.Header("Cache-Control", "must-revalidate")
	c.Header("Pragma", "public")

	c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", templateData)
}
