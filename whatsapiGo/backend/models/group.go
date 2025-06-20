package models

import (
    "time"
    "gorm.io/gorm"
)

// Group - Modelo de grupo con soporte LID
type Group struct {
    ID               uint           `json:"id" gorm:"primaryKey"`
    InstanceID       string         `json:"instance_id" gorm:"not null;index"`
    JID              string         `json:"jid" gorm:"not null;unique;index"`
    LID              string         `json:"lid" gorm:"index"`
    Name             string         `json:"name" gorm:"not null"`
    Description      string         `json:"description"`
    OwnerJID         string         `json:"owner_jid" gorm:"not null"`
    ParticipantCount int            `json:"participant_count" gorm:"default:0"`
    AvatarID         string         `json:"avatar_id"`
    InviteLink       string         `json:"invite_link"`
    IsAdmin          bool           `json:"is_admin" gorm:"default:false"`
    IsOwner          bool           `json:"is_owner" gorm:"default:false"`
    IsMember         bool           `json:"is_member" gorm:"default:true"`
    GroupCreatedAt   time.Time      `json:"group_created_at"`
    CreatedAt        time.Time      `json:"created_at"`
    UpdatedAt        time.Time      `json:"updated_at"`
    DeletedAt        gorm.DeletedAt `json:"deleted_at" gorm:"index"`
}

func (Group) TableName() string {
    return "groups"
}

// GroupParticipant - Modelo de participantes de grupo
type GroupParticipant struct {
    ID         uint      `json:"id" gorm:"primaryKey"`
    GroupJID   string    `json:"group_jid" gorm:"not null;index"`
    UserJID    string    `json:"user_jid" gorm:"not null;index"`
    UserLID    string    `json:"user_lid"`
    Phone      string    `json:"phone"`
    Name       string    `json:"name"`
    PushName   string    `json:"push_name"`
    IsAdmin    bool      `json:"is_admin" gorm:"default:false"`
    IsOwner    bool      `json:"is_owner" gorm:"default:false"`
    JoinedAt   time.Time `json:"joined_at"`
    AddedBy    string    `json:"added_by"`
    CreatedAt  time.Time `json:"created_at"`
    UpdatedAt  time.Time `json:"updated_at"`
}

func (GroupParticipant) TableName() string {
    return "group_participants"
}

// GroupSettings - Configuraciones del grupo
type GroupSettings struct {
    ID                      uint   `json:"id" gorm:"primaryKey"`
    GroupJID                string `json:"group_jid" gorm:"not null;unique;index"`
    OnlyAdminsCanMessage    bool   `json:"only_admins_can_message" gorm:"default:false"`
    OnlyAdminsCanEditInfo   bool   `json:"only_admins_can_edit_info" gorm:"default:true"`
    OnlyAdminsCanAddMembers bool   `json:"only_admins_can_add_members" gorm:"default:true"`
    ApprovalMode            bool   `json:"approval_mode" gorm:"default:false"`
    Ephemeral               string `json:"ephemeral"`
    MaxSize                 int    `json:"max_size" gorm:"default:512"`
    IsAnnounce              bool   `json:"is_announce" gorm:"default:false"`
    CreatedAt               time.Time `json:"created_at"`
    UpdatedAt               time.Time `json:"updated_at"`
}

func (GroupSettings) TableName() string {
    return "group_settings"
}
