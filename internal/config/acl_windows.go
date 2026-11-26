//go:build windows

package config

import (
	"fmt"

	"golang.org/x/sys/windows"
)

// RestrictDirToCurrentUser applies a protected Windows NTFS DACL granting full control
// exclusively to the current Windows user SID and NT AUTHORITY\SYSTEM.
func RestrictDirToCurrentUser(path string) error {
	tok, err := windows.OpenCurrentProcessToken()
	if err != nil {
		return fmt.Errorf("open current process token: %w", err)
	}
	defer tok.Close()

	tu, err := tok.GetTokenUser()
	if err != nil {
		return fmt.Errorf("get token user SID: %w", err)
	}

	systemSID, err := windows.CreateWellKnownSid(windows.WinLocalSystemSid)
	if err != nil {
		return fmt.Errorf("create SYSTEM SID: %w", err)
	}

	entries := []windows.EXPLICIT_ACCESS{
		{
			AccessPermissions: windows.GENERIC_ALL,
			AccessMode:        windows.SET_ACCESS,
			Inheritance:       windows.SUB_CONTAINERS_AND_OBJECTS_INHERIT,
			Trustee: windows.TRUSTEE{
				TrusteeForm:  windows.TRUSTEE_IS_SID,
				TrusteeType:  windows.TRUSTEE_IS_USER,
				TrusteeValue: windows.TrusteeValueFromSID(tu.User.Sid),
			},
		},
		{
			AccessPermissions: windows.GENERIC_ALL,
			AccessMode:        windows.SET_ACCESS,
			Inheritance:       windows.SUB_CONTAINERS_AND_OBJECTS_INHERIT,
			Trustee: windows.TRUSTEE{
				TrusteeForm:  windows.TRUSTEE_IS_SID,
				TrusteeType:  windows.TRUSTEE_IS_WELL_KNOWN_GROUP,
				TrusteeValue: windows.TrusteeValueFromSID(systemSID),
			},
		},
	}

	dacl, err := windows.ACLFromEntries(entries, nil)
	if err != nil {
		return fmt.Errorf("build NTFS DACL: %w", err)
	}

	err = windows.SetNamedSecurityInfo(
		path,
		windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
		nil,
		nil,
		dacl,
		nil,
	)
	if err != nil {
		return fmt.Errorf("apply NTFS DACL to %s: %w", path, err)
	}
	return nil
}
