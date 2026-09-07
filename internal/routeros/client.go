package routeros

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"

	"git.tai.pe/homelab/mikrotik-adguard-identity/internal/models"
	"git.tai.pe/homelab/mikrotik-adguard-identity/internal/util"
)

type Client struct {
	Host           string
	Port           int
	User, Password string
	Timeout        time.Duration
}

type sentence []string

func (c Client) dial() (net.Conn, error) {
	return net.DialTimeout("tcp", net.JoinHostPort(c.Host, strconv.Itoa(c.Port)), c.Timeout)
}

func (c Client) command(words ...string) ([]map[string]string, error) {
	conn, err := c.dial()
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(c.Timeout))
	r := bufio.NewReader(conn)
	w := bufio.NewWriter(conn)
	if err := writeSentence(w, []string{"/login", "=name=" + c.User, "=password=" + c.Password}); err != nil {
		return nil, err
	}
	if _, err := readUntilDone(r); err != nil {
		return nil, fmt.Errorf("login: %w", err)
	}
	if err := writeSentence(w, words); err != nil {
		return nil, err
	}
	return readUntilDone(r)
}

func routerOSTruthy(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func (c Client) ActiveSessions() ([]models.RadiusSession, error) {
	rows, err := c.command("/user-manager/session/print", "=.proplist=acct-session-id,active,calling-station-id,ended,nas-identifier,nas-ip-address,nas-port-id,user")
	if err != nil {
		return nil, err
	}
	out := []models.RadiusSession{}
	for _, r := range rows {
		activeValue := strings.TrimSpace(r["active"])
		active := routerOSTruthy(activeValue) || (activeValue == "" && r["ended"] == "")
		if !active {
			continue
		}
		if r["acct-session-id"] == "" || r["user"] == "" || r["calling-station-id"] == "" {
			continue
		}
		out = append(out, models.RadiusSession{
			SessionID: r["acct-session-id"], Username: util.NormalizeUsername(r["user"]),
			MAC: util.NormalizeMAC(r["calling-station-id"]), NASID: r["nas-identifier"],
			NASIP: r["nas-ip-address"], NASPortID: r["nas-port-id"],
		})
	}
	return out, nil
}

func (c Client) BoundLeases() ([]models.DHCPBinding, error) {
	rows, err := c.command("/ip/dhcp-server/lease/print", "=.proplist=address,mac-address,status")
	if err != nil {
		return nil, err
	}
	out := []models.DHCPBinding{}
	for _, r := range rows {
		if !strings.EqualFold(r["status"], "bound") || r["address"] == "" || r["mac-address"] == "" {
			continue
		}
		out = append(out, models.DHCPBinding{IP: r["address"], MAC: util.NormalizeMAC(r["mac-address"])})
	}
	return out, nil
}

func writeSentence(w *bufio.Writer, words []string) error {
	for _, word := range words {
		if err := writeWord(w, []byte(word)); err != nil {
			return err
		}
	}
	if err := writeWord(w, nil); err != nil {
		return err
	}
	return w.Flush()
}
func writeWord(w io.Writer, b []byte) error {
	if err := writeLen(w, len(b)); err != nil {
		return err
	}
	if len(b) > 0 {
		_, err := w.Write(b)
		return err
	}
	return nil
}
func writeLen(w io.Writer, n int) error {
	var b [5]byte
	switch {
	case n < 0x80:
		b[0] = byte(n)
		_, e := w.Write(b[:1])
		return e
	case n < 0x4000:
		v := uint16(n) | 0x8000
		b[0] = byte(v >> 8)
		b[1] = byte(v)
		_, e := w.Write(b[:2])
		return e
	case n < 0x200000:
		v := uint32(n) | 0xC00000
		b[0] = byte(v >> 16)
		b[1] = byte(v >> 8)
		b[2] = byte(v)
		_, e := w.Write(b[:3])
		return e
	case n < 0x10000000:
		v := uint32(n) | 0xE0000000
		binary.BigEndian.PutUint32(b[:4], v)
		_, e := w.Write(b[:4])
		return e
	default:
		b[0] = 0xF0
		binary.BigEndian.PutUint32(b[1:5], uint32(n))
		_, e := w.Write(b[:5])
		return e
	}
}
func readLen(r *bufio.Reader) (int, error) {
	b, err := r.ReadByte()
	if err != nil {
		return 0, err
	}
	switch {
	case b&0x80 == 0:
		return int(b), nil
	case b&0xC0 == 0x80:
		b2, e := r.ReadByte()
		if e != nil {
			return 0, e
		}
		return int(b&^0xC0)<<8 | int(b2), nil
	case b&0xE0 == 0xC0:
		x, e := readN(r, 2)
		if e != nil {
			return 0, e
		}
		return int(b&^0xE0)<<16 | int(x[0])<<8 | int(x[1]), nil
	case b&0xF0 == 0xE0:
		x, e := readN(r, 3)
		if e != nil {
			return 0, e
		}
		return int(b&^0xF0)<<24 | int(x[0])<<16 | int(x[1])<<8 | int(x[2]), nil
	case b == 0xF0:
		x, e := readN(r, 4)
		if e != nil {
			return 0, e
		}
		return int(binary.BigEndian.Uint32(x)), nil
	default:
		return 0, fmt.Errorf("invalid RouterOS length prefix")
	}
}
func readN(r io.Reader, n int) ([]byte, error) {
	b := make([]byte, n)
	_, e := io.ReadFull(r, b)
	return b, e
}
func readSentence(r *bufio.Reader) (sentence, error) {
	var s sentence
	for {
		n, e := readLen(r)
		if e != nil {
			return nil, e
		}
		if n == 0 {
			return s, nil
		}
		b, e := readN(r, n)
		if e != nil {
			return nil, e
		}
		s = append(s, string(b))
	}
}
func readUntilDone(r *bufio.Reader) ([]map[string]string, error) {
	rows := []map[string]string{}
	for {
		s, e := readSentence(r)
		if e != nil {
			return nil, e
		}
		if len(s) == 0 {
			continue
		}
		switch s[0] {
		case "!re":
			m := map[string]string{}
			for _, w := range s[1:] {
				if strings.HasPrefix(w, "=") {
					kv := strings.SplitN(w[1:], "=", 2)
					if len(kv) == 2 {
						m[kv[0]] = kv[1]
					}
				}
			}
			rows = append(rows, m)
		case "!trap", "!fatal":
			return nil, fmt.Errorf("RouterOS API error: %s", strings.Join(s[1:], " "))
		case "!done":
			return rows, nil
		}
	}
}
