package ftp

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"net/textproto"
	"strconv"
	"strings"
	"time"
)

type ServerConn struct {
	conn *textproto.Conn
	host string
}

type response struct {
	conn net.Conn
	c    *ServerConn
}

// Connect to a ftp server and returns a ServerConn handler.
func Connect(addr string) (*ServerConn, error) {
	if strings.Contains(addr, ":") == false {
		addr = addr + ":21"
	}
	conn, err := textproto.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}

	a := strings.SplitN(addr, ":", 2)
	c := &ServerConn{conn, a[0]}

	_, _, err = c.conn.ReadCodeLine(StatusReady)
	if err != nil {
		c.Quit()
		return nil, err
	}

	return c, nil
}

func (c *ServerConn) Login(user, password string) error {
	_, _, err := c.cmd(StatusUserOK, "USER %s", user)
	if err != nil {
		return err
	}

	code, _, err := c.cmd(StatusLoggedIn, "PASS %s", password)
	if code == StatusLoggedIn {
		return nil
	}
	return err
}

// Enter passive mode
func (c *ServerConn) pasv() (port int, err error) {
	c.conn.Cmd("PASV")
	code, line, err := c.conn.ReadCodeLine(StatusExtendedPassiveMode)
	if (err != nil) && (code != StatusPassiveMode) {
		return
	} else {
		err = nil
	}
	start, end := strings.Index(line, "("), strings.Index(line, ")")
	if start == -1 || end == -1 {
		err = errors.New("Invalid PASV response format")
		return
	}
	s := strings.Split(line[start+1:end], ",")
	l1, _ := strconv.Atoi(s[len(s)-2])
	l2, _ := strconv.Atoi(s[len(s)-1])
	port = l1*256 + l2
	return
}

// Enter extended passive mode
func (c *ServerConn) epsv() (port int, err error) {
	c.conn.Cmd("EPSV")
	_, line, err := c.conn.ReadCodeLine(StatusExtendedPassiveMode)
	if err != nil {
		return
	}
	start := strings.Index(line, "|||")
	end := strings.LastIndex(line, "|")
	if start == -1 || end == -1 {
		err = errors.New("Invalid EPSV response format")
		return
	}
	port, err = strconv.Atoi(line[start+3 : end])
	return
}

// Open a new data connection using passive mode
func (c *ServerConn) openDataConn() (net.Conn, error) {
	port, err := c.pasv()
	port, err = c.pasv()
	port, err = c.pasv()
	if err != nil {
		return nil, err
	}

	// Build the new net address string
	addr := fmt.Sprintf("%s:%d", c.host, port)
	conn, err := net.DialTimeout("tcp", addr, time.Duration(2400)*time.Second)
	if err != nil {
		return nil, err
	}

	return conn, nil
}

// Helper function to execute a command and check for the expected code
func (c *ServerConn) cmd(expected int, format string, args ...interface{}) (int, string, error) {
	_, err := c.conn.Cmd(format, args...)
	if err != nil {
		return 0, "", err
	}
	code, line, err := c.conn.ReadCodeLine(expected)
	for code == StatusLoggedIn && expected == StatusPathCreated {
		code, line, err = c.conn.ReadCodeLine(expected)
	}
	return code, line, err
}

// Helper function to execute commands which require a data connection
func (c *ServerConn) cmdDataConn(format string, args ...interface{}) (net.Conn, error) {
	conn, err := c.openDataConn()
	if err != nil {
		return nil, err
	}
	
	_, err = c.conn.Cmd(format, args...)
	if err != nil {
		conn.Close()
		return nil, err
	}

	code, msg, err := c.conn.ReadCodeLine(-1)
	if err != nil {
		conn.Close()
		return nil, err
	}
	if code != StatusAlreadyOpen && code != StatusAboutToSend && code != StatusPassiveMode {
	 	conn.Close()
	 	return nil, &textproto.Error{code, msg}
	}

	return conn, nil
}

func (c *ServerConn) List(path string) (entries []*FTPListData, err error) {
	conn, err := c.cmdDataConn("LIST %s", path)
	//fmt.Println("List : err = ", err)
	if err != nil {
		return
	}

	r := &response{conn, c}
	defer r.Close()

	bio := bufio.NewReader(r)
	for {
		line, e := bio.ReadString('\n')
		if e == io.EOF {
			break
		} else if e != nil {
			return nil, e
		}
		ftplistdata := ParseLine(line)
		entries = append(entries, ftplistdata)
	}
	return
}

// Changes the current directory to the specified path.
func (c *ServerConn) ChangeDir(path string) error {
	_, _, err := c.cmd(StatusRequestedFileActionOK, "CWD %s", path)
	return err
}

// Changes the current directory to the parent directory.
// ChangeDir("..")
func (c *ServerConn) ChangeDirToParent() error {
	_, _, err := c.cmd(StatusRequestedFileActionOK, "CDUP")
	return err
}

// Returns the path of the current directory.
func (c *ServerConn) CurrentDir() (string, error) {
	_, msg, err := c.cmd(StatusPathCreated, "PWD")
	if err != nil {
		//fmt.Println("PWD err : ", err, "msg : ", msg)
		return "", err
	}
	//fmt.Println("PWD success")
	start := strings.Index(msg, "\"")
	end := strings.LastIndex(msg, "\"")

	if start == -1 || end == -1 {
		return "", errors.New("Unsuported PWD response format")
	}

	return msg[start+1 : end], nil
}

// Retrieves a file from the remote FTP server.
// The ReadCloser must be closed at the end of the operation.
func (c *ServerConn) Retr(path string) (io.ReadCloser, error) {
	conn, err := c.cmdDataConn("RETR %s", path)
	if err != nil {
		return nil, err
	}

	r := &response{conn, c}
	return r, nil
}

// Uploads a file to the remote FTP server.
// This function gets the data from the io.Reader. Hint: io.Pipe()
func (c *ServerConn) Stor(path string, r io.Reader) error {
	conn, err := c.cmdDataConn("STOR %s", path)
	if err != nil {
		return err
	}

	_, err = io.Copy(conn, r)
	conn.Close()
	if err != nil {
		return err
	}

	_, _, err = c.conn.ReadCodeLine(StatusClosingDataConnection)
	return err
}

// Renames a file on the remote FTP server.
func (c *ServerConn) Rename(from, to string) error {
	_, _, err := c.cmd(StatusRequestFilePending, "RNFR %s", from)
	if err != nil {
		return err
	}

	_, _, err = c.cmd(StatusRequestedFileActionOK, "RNTO %s", to)
	return err
}

// Deletes a file on the remote FTP server.
func (c *ServerConn) Delete(path string) error {
	_, _, err := c.cmd(StatusRequestedFileActionOK, "DELE %s", path)
	return err
}

// Creates a new directory on the remote FTP server.
func (c *ServerConn) MakeDir(path string) error {
	_, _, err := c.cmd(StatusPathCreated, "MKD %s", path)
	return err
}

// Removes a directory from the remote FTP server.
func (c *ServerConn) RemoveDir(path string) error {
	_, _, err := c.cmd(StatusRequestedFileActionOK, "RMD %s", path)
	return err
}

// Sends a NOOP command. Usualy used to prevent timeouts.
func (c *ServerConn) NoOp() error {
	_, _, err := c.cmd(StatusCommandOK, "NOOP")
	return err
}

// Properly close the connection from the remote FTP server.
// It notifies the remote server that we are about to close the connection,
// then it really closes it.
func (c *ServerConn) Quit() error {
	c.conn.Cmd("QUIT")
	return c.conn.Close()
}

func (r *response) Read(buf []byte) (int, error) {
	n, err := r.conn.Read(buf)
	if err == io.EOF {
		code, _, err2 := r.c.conn.ReadCodeLine(StatusClosingDataConnection)
		if (err2 != nil) && (code != StatusPassiveMode) {
			err = err2
		}
	}
	return n, err
}

func (r *response) Close() error {
	return r.conn.Close()
}
