package service

import (
	"errors"
	"strings"

	"gorm.io/gorm"

	"physics-lab/backend/internal/model"
	"physics-lab/backend/internal/pkg/jwt"
	"physics-lab/backend/internal/pkg/wechat"
	"physics-lab/backend/internal/repository"
)

// LoginUser 登录响应里的 user 对象（契约 §1）
type LoginUser struct {
	ID           int64   `json:"id"`
	OpenID       string  `json:"openid"`
	Role         string  `json:"role"`
	Name         *string `json:"name"`
	StudentNo    *string `json:"student_no"`
	NeedComplete bool    `json:"need_complete"`
}

type AuthService struct {
	userRepo          *repository.UserRepository
	classRepo         *repository.ClassRepository
	jm                *jwt.Manager
	wx                *wechat.Client
	allowDevBackdoor  bool
	teacherInviteCode string
}

func NewAuthService(userRepo *repository.UserRepository, classRepo *repository.ClassRepository, jm *jwt.Manager, wx *wechat.Client, allowDevBackdoor bool, teacherInviteCode string) *AuthService {
	return &AuthService{
		userRepo: userRepo, classRepo: classRepo, jm: jm, wx: wx,
		allowDevBackdoor:  allowDevBackdoor,
		teacherInviteCode: teacherInviteCode,
	}
}

// LoginInput 登录入参
type LoginInput struct {
	Code       string // wx.login code 或 dev_ 后门码
	Role       string // "student" / "teacher" / "admin"（空则默认 student）
	InviteCode string // 教师邀请码（role=teacher 时必填）
}

// Login 用 wx.login code 换 token。
// code 以 dev_ 开头走后门（仅 allow_dev_backdoor=true 时；上线前关闭开关）。
// role=teacher 时需校验 invite_code（静态配置）。
// role=admin 时不允许自助注册，必须已在 DB 中且 role=admin。
func (s *AuthService) Login(input LoginInput) (token string, user *LoginUser, err error) {
	code := strings.TrimSpace(input.Code)
	if code == "" {
		return "", nil, errors.New("code 不能为空")
	}

	var openid, role string
	if strings.HasPrefix(code, "dev_") {
		if !s.allowDevBackdoor {
			return "", nil, errors.New("code 校验失败")
		}
		openid, role = devBackdoor(code)
	} else {
		// 正常微信登录：角色由前端选择决定
		role = strings.TrimSpace(input.Role)
		if role == "" {
			role = "student"
		}
		if role == "teacher" {
			if input.InviteCode == "" {
				return "", nil, errors.New("教师注册需填写邀请码")
			}
			if s.teacherInviteCode == "" || input.InviteCode != s.teacherInviteCode {
				return "", nil, errors.New("邀请码错误")
			}
		}
		if role == "admin" {
			// 管理员不能自助注册：wx.login 换到 openid 后，必须已在 DB 中且 role=admin
			sess, err := s.wx.Code2Session(code)
			if err != nil {
				return "", nil, err
			}
			openid = sess.OpenID
			u, err := s.userRepo.FindByOpenID(openid)
			if err != nil || u.Role != "admin" {
				return "", nil, errors.New("该账号不是管理员，请联系系统管理员开通")
			}
			token, err := s.jm.Sign(u.ID, u.Role)
			if err != nil {
				return "", nil, err
			}
			return token, toLoginUser(u), nil
		}
		if role != "student" && role != "teacher" {
			return "", nil, errors.New("无效的角色")
		}
		sess, err := s.wx.Code2Session(code)
		if err != nil {
			return "", nil, err
		}
		openid = sess.OpenID
	}

	u, err := s.userRepo.FindByOpenID(openid)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return "", nil, err
		}
		// 首次登录：建用户，未补全学籍
		u = &model.User{OpenID: openid, Role: role}
		if err := s.userRepo.Create(u); err != nil {
			return "", nil, err
		}
	}

	token, err = s.jm.Sign(u.ID, u.Role)
	if err != nil {
		return "", nil, err
	}
	return token, toLoginUser(u), nil
}

// Me 当前登录用户
func (s *AuthService) Me(userID int64) (*LoginUser, error) {
	u, err := s.userRepo.FindByID(userID)
	if err != nil {
		return nil, err
	}
	return toLoginUser(u), nil
}

// CompleteProfile 补全学籍信息；class_id>0 时同时加入班级
func (s *AuthService) CompleteProfile(userID int64, name, studentNo string, classID *int64) (*model.User, error) {
	u, err := s.userRepo.UpdateProfileFields(userID, name, studentNo)
	if err != nil {
		return nil, err
	}
	if classID != nil && *classID > 0 {
		if err := s.classRepo.AddMember(*classID, userID); err != nil && !errors.Is(err, repository.ErrAlreadyMember) {
			// 加班级失败不阻断补全，仅记录
			_ = err
		}
	}
	return u, nil
}

func toLoginUser(u *model.User) *LoginUser {
	return &LoginUser{
		ID:           u.ID,
		OpenID:       u.OpenID,
		Role:         u.Role,
		Name:         u.Name,
		StudentNo:    u.StudentNo,
		NeedComplete: u.Name == nil || *u.Name == "",
	}
}

// devBackdoor 开发期后门：dev_student/dev_teacher/dev_admin -> 内置测试账号
func devBackdoor(code string) (openid, role string) {
	switch code {
	case "dev_student":
		return "oDEV_STUDENT", "student"
	case "dev_teacher":
		return "oDEV_TEACHER", "teacher"
	case "dev_admin":
		return "oDEV_ADMIN", "admin"
	default:
		return "oDEV_" + strings.TrimPrefix(code, "dev_"), "student"
	}
}
