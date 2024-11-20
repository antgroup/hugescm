package git

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/antgroup/hugescm/modules/command"
)

const (
	// contentsCommand is the command expected by the `--batch-command` mode of git-cat-file(1)
	// for reading an objects contents.
	contentsCommand = "contents"
	// infoCommand is the command expected by the `--batch-command` mode of git-cat-file(1)
	// for reading an objects info.
	infoCommand = "info"
	// Used with --buffer to execute all preceding commands that were issued since the beginning or since the last flush was issued.
	// When --buffer is used, no output will come until a flush is issued.
	// When --buffer is not used, commands are flushed each time without issuing flush.
	flushCommand = "flush"
)

type Decoder struct {
	stdout  *bufio.Reader
	stdin   *bufio.Writer
	cleanup func()
}

func NewDecoder(ctx context.Context, repoPath string) (*Decoder, error) {
	stderr := command.NewStderr()
	cmd := command.NewFromOptions(ctx, &command.RunOpts{Stderr: stderr},
		"git", "--git-dir", repoPath, "cat-file", "--batch-command", "--buffer")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		_ = stdout.Close()
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		_ = stdout.Close()
		_ = stdin.Close()
		return nil, err
	}
	return &Decoder{
		stdout: bufio.NewReader(stdout),
		stdin:  bufio.NewWriter(stdin),
		cleanup: func() {
			_ = stdin.Close()
			_ = stdout.Close()
			_ = cmd.Wait()
			// if err := cmd.Wait(); err != nil {
			// 	logrus.Infof("stderr: %s", stderr.String())
			// }
		}}, nil
}

func (d *Decoder) Close() error {
	if d.cleanup != nil {
		d.cleanup()
	}
	return nil
}

func (d *Decoder) flush() error {
	if _, err := d.stdin.WriteString(flushCommand); err != nil {
		return fmt.Errorf("writing flush command: %w", err)
	}

	if err := d.stdin.WriteByte('\n'); err != nil {
		return fmt.Errorf("terminating flush command: %w", err)
	}

	if err := d.stdin.Flush(); err != nil {
		return fmt.Errorf("flushing: %w", err)
	}
	return nil
}

func (d *Decoder) readObject(cmd, revision string) error {
	if strings.IndexByte(revision, '\n') != -1 {
		return NewObjectNotFound(revision)
	}
	if _, err := d.stdin.WriteString(cmd); err != nil {
		return fmt.Errorf("writing cmd request: %w", err)
	}
	if err := d.stdin.WriteByte(' '); err != nil {
		return fmt.Errorf("terminating object request: %w", err)
	}
	if _, err := d.stdin.WriteString(revision); err != nil {
		return fmt.Errorf("writing object request: %w", err)
	}
	if err := d.stdin.WriteByte('\n'); err != nil {
		return fmt.Errorf("terminating object request: %w", err)
	}
	return nil
}

const (
	missingSuffix = " missing"
)

// readBatchLine reads the header line from cat-file --batch-command -z --buffer
// We expect:
// <sha> SP <type> SP <size> LF
// sha is a 40/64byte not 20/32byte here
func (d *Decoder) readBatchLine() (string, string, int64, error) {
	line, err := d.stdout.ReadString('\n')
	if err != nil {
		return "", "", 0, err
	}
	if len(line) == 1 {
		if line, err = d.stdout.ReadString('\n'); err != nil {
			return "", "", 0, err
		}
	}
	line = strings.TrimSuffix(line, "\n")
	if strings.HasSuffix(line, missingSuffix) {
		return "", "", 0, NewObjectNotFound(line[0 : len(line)-len(missingSuffix)])
	}
	pos := strings.IndexByte(line, ' ')
	if pos < 0 {
		return "", "", 0, NewObjectNotFound(line)
	}
	sha := line[:pos]
	t, sizeSz, ok := strings.Cut(line[pos+1:], " ")
	if !ok {
		return "", "", 0, NewObjectNotFound(sha)
	}
	size, err := strconv.ParseInt(sizeSz, 10, 64)
	return sha, t, size, err
}

func (d *Decoder) Meta(objectKey string) (*Metadata, error) {
	if err := d.readObject(infoCommand, objectKey); err != nil {
		return nil, err
	}
	if err := d.flush(); err != nil {
		return nil, err
	}
	oid, objectType, size, err := d.readBatchLine()
	if err != nil {
		return nil, err
	}
	t, _ := ParseObjectType(objectType)
	return &Metadata{Hash: oid, Type: t, Size: size}, nil
}

func (d *Decoder) object(objectKey string) (*Object, error) {
	if err := d.readObject(contentsCommand, objectKey); err != nil {
		return nil, err
	}
	if err := d.flush(); err != nil {
		return nil, err
	}
	oid, objectType, size, err := d.readBatchLine()
	if err != nil {
		return nil, err
	}
	r := io.LimitReader(d.stdout, size)
	t, _ := ParseObjectType(objectType)
	return &Object{Hash: oid, Size: size, Type: t, dataReader: r}, nil
}

func (d *Decoder) ObjectReader(objectKey string) (*Object, error) {
	return d.object(objectKey)
}

func (d *Decoder) Object(objectKey string) (any, error) {
	obj, err := d.object(objectKey)
	if err != nil {
		return nil, err
	}
	if obj.Type == BlobObject {
		return obj, nil
	}
	defer obj.Discard()
	switch obj.Type {
	case CommitObject:
		commit := new(Commit)
		if err := commit.Decode(obj.Hash, obj); err != nil {
			return nil, err
		}
		return commit, nil
	case TagObject:
		tag := new(Tag)
		if err := tag.Decode(obj.Hash, obj); err != nil {
			return nil, err
		}
		return tag, nil
	case TreeObject:
		tree := new(Tree)
		if _, err := tree.Decode(obj.Hash, obj); err != nil {
			return nil, err
		}
		return tree, nil
	default:
	}
	return nil, &ErrUnexpectedType{message: fmt.Sprintf("unexoected object '%s' type: %s", objectKey, obj.Type)}
}

func (d *Decoder) Tree(objectKey string) (*Tree, error) {
	obj, err := d.object(objectKey)
	if err != nil {
		return nil, err
	}
	defer obj.Discard()
	if obj.Type != TreeObject {
		return nil, &ErrUnexpectedType{message: fmt.Sprintf("object '%s' type is '%s' not tree", objectKey, obj.Type)}
	}
	tree := new(Tree)
	if _, err := tree.Decode(obj.Hash, obj); err != nil {
		return nil, err
	}
	return tree, nil
}

func (d *Decoder) Commit(objectKey string) (*Commit, error) {
	obj, err := d.object(objectKey)
	if err != nil {
		return nil, err
	}
	defer obj.Discard()
	if obj.Type != CommitObject {
		return nil, &ErrUnexpectedType{message: fmt.Sprintf("object '%s' type is '%s' not commit", objectKey, obj.Type)}
	}
	commit := new(Commit)
	if err := commit.Decode(obj.Hash, obj); err != nil {
		return nil, err
	}
	return commit, nil
}

func (d *Decoder) Blob(objectKey string) (*Object, error) {
	obj, err := d.object(objectKey)
	if err != nil {
		return nil, err
	}
	if obj.Type != BlobObject {
		obj.Discard()
		return nil, &ErrUnexpectedType{message: fmt.Sprintf("object '%s' type is '%s' not blob", objectKey, obj.Type)}
	}
	return obj, nil
}

func (d *Decoder) ReadOverflow(objectKey string, limit int64) (b []byte, err error) {
	br, err := d.Blob(objectKey)
	if err != nil {
		return nil, err
	}
	defer br.Discard()
	if limit > 0 && br.Size > limit {
		return nil, errors.New("reading file size limit exceeded")
	}
	b, err = io.ReadAll(br.dataReader)
	return
}

func (d *Decoder) BlobEntry(revision string, path string) (*Object, error) {
	return d.Blob(revision + ":" + path)
}

func (d *Decoder) ReadEntry(revision string, path string) (*Object, error) {
	return d.ObjectReader(revision + ":" + path)
}

// ResolveCommit resolve peeled commit
func (d *Decoder) ResolveCommit(objectKey string) (*Commit, error) {
	oid := objectKey
	for {
		obj, err := d.object(oid)
		if err != nil {
			return nil, err
		}
		switch obj.Type {
		case CommitObject:
			commit := new(Commit)
			if err := commit.Decode(obj.Hash, obj); err != nil {
				return nil, err
			}
			return commit, nil
		case TagObject:
			tag := new(Tag)
			if err := tag.Decode(obj.Hash, obj); err != nil {
				return nil, err
			}
			oid = tag.Object
		default:
			obj.Discard()
			return nil, &ErrUnexpectedType{message: fmt.Sprintf("object '%s' type is '%s' not commit", oid, obj.Type)}
		}
	}
}

func (d *Decoder) ExhaustiveMeta(location string) (*Metadata, error) {
	bs := []byte(location)
	for i := 0; i < len(location); {
		pos := bytes.IndexByte(bs[i:], '/')
		if pos == -1 {
			return d.Meta(location)
		}
		bs[pos+i] = ':'
		m, err := d.Meta(string(bs))
		if err == nil {
			return m, nil
		}
		if !IsErrNotExist(err) {
			return nil, err
		}
		bs[pos+i] = '/'
		i += pos + 1
	}
	return nil, NewObjectNotFound(location)
}

// ExhaustiveObjectReader: Exhaustive read object
//
// Can two branches 'a' and 'a/b' exist at the same time in git? Normally, this is impossible,
// but when we manually edit packed-refs, we can create 'a' and 'a/b' at the same time,
// because packed-refs has no file system restrictions, of course this will Annoys git,
// so it's not recommended, in the 'Exhaustive*' functions, we don't care about this unusual case.
func (d *Decoder) ExhaustiveObjectReader(location string) (*Object, error) {
	bs := []byte(location)
	for i := 0; i < len(location); {
		pos := bytes.IndexByte(bs[i:], '/')
		if pos == -1 {
			return d.ObjectReader(location)
		}
		bs[pos+i] = ':'
		obj, err := d.ObjectReader(string(bs))
		if err == nil {
			return obj, nil
		}
		if !IsErrNotExist(err) {
			return nil, err
		}
		bs[pos+i] = '/'
		i += pos + 1
	}
	return nil, NewObjectNotFound(location)
}
