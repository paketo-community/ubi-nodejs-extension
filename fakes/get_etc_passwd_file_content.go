package fakes

func Get_etc_passwd_file_content(etcPasswdFileContent string) func() (string, error) {
	return func() (string, error) {
		return etcPasswdFileContent, nil
	}
}
