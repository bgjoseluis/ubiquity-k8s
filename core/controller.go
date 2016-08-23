package core

import (
	"fmt"
	"log"

	common "github.ibm.com/almaden-containers/spectrum-common.git/core"
	"github.ibm.com/almaden-containers/spectrum-common.git/models"
)

type Controller struct {
	Client common.SpectrumClient
	log    *log.Logger
}

func NewController(logger *log.Logger, filesystem, mountpath string) *Controller {
	return &Controller{log: logger, Client: common.NewSpectrumClient(logger, filesystem, mountpath)}
}

func NewControllerWithClient(logger *log.Logger, client common.SpectrumClient) *Controller {
	return &Controller{log: logger, Client: client}
}

func (c *Controller) Init() *models.FlexVolumeResponse {
	c.log.Println("controller-activate-start")
	defer c.log.Println("controller-activate-end")

	return &models.FlexVolumeResponse{
		Status:  "Success",
		Message: "Plugin init successfully",
		Device:  "",
	}
}

func (c *Controller) Attach(attachRequest *models.FlexVolumeAttachRequest) *models.FlexVolumeResponse {
	c.log.Println("controller-attach-start")
	defer c.log.Println("controller-attach-end")
	c.log.Printf("attach-details %#v\n", attachRequest)

	opts := map[string]interface{}{"fileset": attachRequest.FileSet}
	err := c.Client.Create(attachRequest.VolumeId, opts)
	var attachResponse *models.FlexVolumeResponse
	if err != nil {
		attachResponse = &models.FlexVolumeResponse{
			Status:  "Failure",
			Message: fmt.Sprintf("Failed to attach volume: %#v", err),
			Device:  attachRequest.VolumeId,
		}
	} else {
		attachResponse = &models.FlexVolumeResponse{
			Status:  "Success",
			Message: "Volume attached successfully",
			Device:  attachRequest.VolumeId,
		}
	}
	return attachResponse
}

func (c *Controller) Detach(detachRequest *models.GenericRequest) *models.FlexVolumeResponse {
	c.log.Println("controller-detach-start")
	defer c.log.Println("controller-detach-end")

	existingVolume, _, err := c.Client.Get(detachRequest.Name)

	if err != nil && err.Error() != "Cannot find info" {
		return &models.FlexVolumeResponse{
			Status:  "Failure",
			Message: fmt.Sprintf("Failed to detach volume %#v", err),
			Device:  detachRequest.Name,
		}
	}

	if existingVolume != nil {
		err = c.Client.Remove(detachRequest.Name)
		if err != nil {
			return &models.FlexVolumeResponse{
				Status:  "Failure",
				Message: fmt.Sprintf("Failed to detach volume %#v", err),
				Device:  detachRequest.Name,
			}
		}

		return &models.FlexVolumeResponse{
			Status:  "Success",
			Message: "Volume detached successfully",
			Device:  detachRequest.Name,
		}
	}

	return &models.FlexVolumeResponse{
		Status:  "Failure",
		Message: "Volume not found",
		Device:  detachRequest.Name,
	}
}

func (c *Controller) Mount(mountRequest *models.FlexVolumeMountRequest) *models.FlexVolumeResponse {
	c.log.Println("controller-mount-start")
	defer c.log.Println("controller-mount-end")

	existingVolume, _, err := c.Client.Get(mountRequest.MountDevice)
	if err != nil && err.Error() != "Cannot find info" {
		return &models.FlexVolumeResponse{
			Status:  "Failure",
			Message: fmt.Sprintf("Failed to mount volume %#v", err),
			Device:  "",
		}
	}

	if existingVolume == nil {
		return &models.FlexVolumeResponse{
			Status:  "Failure",
			Message: "Failed to mount volume: volume not found",
			Device:  "",
		}
	}

	mountedPath, err := c.Client.Attach(mountRequest.MountDevice)
	if err != nil {
		return &models.FlexVolumeResponse{
			Status:  "Failure",
			Message: fmt.Sprintf("Failed to mount volume %#v", err),
			Device:  "",
		}
	}

	return &models.FlexVolumeResponse{
		Status:  "Success",
		Message: fmt.Sprintf("Volume mounted successfully to %s", mountedPath),
		Device:  "",
	}
}

func (c *Controller) Unmount(unmountRequest *models.GenericRequest) *models.FlexVolumeResponse {
	c.log.Println("Controller: unmount start")
	defer c.log.Println("Controller: unmount end")

	filesetName, err := c.Client.GetFileSetForMountPoint(unmountRequest.Name)
	if err != nil {
		return &models.FlexVolumeResponse{
			Status:  "Failure",
			Message: fmt.Sprintf("Error finding the volume %#v", err),
			Device:  "",
		}
	}
	if filesetName == "" {
		return &models.FlexVolumeResponse{
			Status:  "Failure",
			Message: "Volume not found",
			Device:  "",
		}
	}

	err = c.Client.Detach(filesetName)
	if err != nil {
		return &models.FlexVolumeResponse{
			Status:  "Failure",
			Message: fmt.Sprintf("Failed to unmount volume %#v", err),
			Device:  "",
		}
	}

	return &models.FlexVolumeResponse{
		Status:  "Success",
		Message: "Volume unmounted successfully",
		Device:  "",
	}
}