//project domain -> responsible for everything related to projects (create project/list projects/update proejct/delete project/get project)
//a user owns a project (uploaded media)


package project 


//a folder/workspace, belongs to one User. Exists purely to organise media (Client Videos, Youtube Drafts). one user can have many projects


//going with explicit fields because learning + clarity  . you could use gorm model schema and that gives you by default all createdAt, updateAt, etc metadata and you just have to specify Name string, Email string, etc etc
import (
	"time"
	"github.com/google/uuid"
)

type Project struct {
	ID     uuid.UUID    `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	Name   string     `gorm:"size:255;not null"`
	Description   string    `gorm:"type:text"`
	CreatedAt    time.Time 
	UpdatedAt    time.Time 
	
}