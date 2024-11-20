package gitobj

import (
	"bytes"
	"crypto/sha1"
	"crypto/sha256"
	"fmt"
	"hash"
	"io"
	"os"
	"sync/atomic"

	"github.com/antgroup/hugescm/modules/git/gitobj/storage"
)

// Database enables the reading and writing of objects against a storage
// backend.
type Database struct {
	// members managed via sync/atomic must be aligned at the top of this
	// structure (see: https://github.com/git-lfs/git-lfs/pull/2880).

	// closed is a uint32 managed by sync/atomic's <X>Uint32 methods. It
	// yields a value of 0 if the *Database it is stored upon is open,
	// and a value of 1 if it is closed.
	closed uint32

	// ro is the locations from which we can read objects.
	ro storage.Storage
	// rw is the location to which we write objects.
	rw storage.WritableStorage

	// temp directory, defaults to os.TempDir
	tmp string

	// objectFormat is the object format (hash algorithm)
	objectFormat ObjectFormatAlgorithm
}

type options struct {
	alternates   string
	objectFormat ObjectFormatAlgorithm
}

type Option func(*options)

type ObjectFormatAlgorithm string

const (
	ObjectFormatSHA1   = ObjectFormatAlgorithm("sha1")
	ObjectFormatSHA256 = ObjectFormatAlgorithm("sha256")
)

// Alternates is an Option to specify the string of alternate repositories that
// are searched for objects.  The format is the same as for
// GIT_ALTERNATE_OBJECT_DIRECTORIES.
func Alternates(alternates string) Option {
	return func(args *options) {
		args.alternates = alternates
	}
}

// ObjectFormat is an Option to specify the hash algorithm (object format) in
// use in Git.  If not specified, it defaults to ObjectFormatSHA1.
func ObjectFormat(algo ObjectFormatAlgorithm) Option {
	return func(args *options) {
		args.objectFormat = algo
	}
}

// NewDatabase constructs an *Database instance that is backed by a
// directory on the filesystem. Specifically, this should point to:
//
//	/absolute/repo/path/.git/objects
func NewDatabase(root, tmp string, setters ...Option) (*Database, error) {
	args := &options{objectFormat: ObjectFormatSHA1}

	for _, setter := range setters {
		setter(args)
	}

	b, err := NewFilesystemBackend(root, tmp, args.alternates, hasher(args.objectFormat))
	if err != nil {
		return nil, err
	}

	odb, err := FromBackend(b, setters...)
	if err != nil {
		return nil, err
	}
	odb.tmp = tmp
	return odb, nil
}

func FromBackend(b storage.Backend, setters ...Option) (*Database, error) {
	args := &options{objectFormat: ObjectFormatSHA1}

	for _, setter := range setters {
		setter(args)
	}

	ro, rw := b.Storage()
	odb := &Database{
		ro:           ro,
		rw:           rw,
		objectFormat: args.objectFormat,
	}
	return odb, nil
}

// Close closes the *Database, freeing any open resources (namely: the
// `*git.ObjectScanner instance), and returning any errors encountered in
// closing them.
//
// If Close() has already been called, this function will return an error.
func (d *Database) Close() error {
	if !atomic.CompareAndSwapUint32(&d.closed, 0, 1) {
		return fmt.Errorf("git/object: *Database already closed")
	}

	if err := d.ro.Close(); err != nil {
		return err
	}
	if err := d.rw.Close(); err != nil {
		return err
	}
	return nil
}

// Object returns an Object (of unknown implementation) satisfying the type
// associated with the object named "sha".
//
// If the object could not be opened, is of unknown type, or could not be
// decoded, than an appropriate error is returned instead.
func (d *Database) Object(sha []byte) (Object, error) {
	r, err := d.open(sha)
	if err != nil {
		return nil, err
	}

	typ, _, err := r.Header()
	if err != nil {
		return nil, err
	}

	var into Object
	switch typ {
	case BlobObjectType:
		into = new(Blob)
	case TreeObjectType:
		into = new(Tree)
	case CommitObjectType:
		into = new(Commit)
	case TagObjectType:
		into = new(Tag)
	default:
		return nil, fmt.Errorf("git/object: unknown object type: %s", typ)
	}
	return into, d.decode(r, into)
}

// Blob returns a *Blob as identified by the SHA given, or an error if one was
// encountered.
func (d *Database) Blob(sha []byte) (*Blob, error) {
	var b Blob

	if err := d.openDecode(sha, &b); err != nil {
		return nil, err
	}
	return &b, nil
}

// Tree returns a *Tree as identified by the SHA given, or an error if one was
// encountered.
func (d *Database) Tree(sha []byte) (*Tree, error) {
	var t Tree
	if err := d.openDecode(sha, &t); err != nil {
		return nil, err
	}
	return &t, nil
}

// Commit returns a *Commit as identified by the SHA given, or an error if one
// was encountered.
func (o *Database) Commit(sha []byte) (*Commit, error) {
	var c Commit

	if err := o.openDecode(sha, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

// Tag returns a *Tag as identified by the SHA given, or an error if one was
// encountered.
func (d *Database) Tag(sha []byte) (*Tag, error) {
	var t Tag

	if err := d.openDecode(sha, &t); err != nil {
		return nil, err
	}
	return &t, nil
}

// WriteBlob stores a *Blob on disk and returns the SHA it is uniquely
// identified by, or an error if one was encountered.
func (d *Database) WriteBlob(b *Blob) ([]byte, error) {
	tmp, err := os.CreateTemp(d.tmp, "")
	if err != nil {
		return nil, err
	}
	defer d.cleanup(tmp)

	to := NewObjectWriter(tmp, d.Hasher())
	if _, err = to.WriteHeader(b.Type(), int64(b.Size)); err != nil {
		return nil, err
	}

	if err = b.Close(); err != nil {
		return nil, err
	}

	if _, err = io.Copy(to, b.Contents); err != nil {
		return nil, err
	}

	if err = to.Close(); err != nil {
		return nil, err
	}

	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	sha, _, err := d.save(to.Sha(), tmp)
	return sha, err
}

// WriteTree stores a *Tree on disk and returns the SHA it is uniquely
// identified by, or an error if one was encountered.
func (o *Database) WriteTree(t *Tree) ([]byte, error) {
	sha, _, err := o.encode(t)
	if err != nil {
		return nil, err
	}
	return sha, nil
}

// WriteCommit stores a *Commit on disk and returns the SHA it is uniquely
// identified by, or an error if one was encountered.
func (o *Database) WriteCommit(c *Commit) ([]byte, error) {
	sha, _, err := o.encode(c)
	if err != nil {
		return nil, err
	}
	return sha, nil
}

// WriteTag stores a *Tag on disk and returns the SHA it is uniquely identified
// by, or an error if one was encountered.
func (o *Database) WriteTag(t *Tag) ([]byte, error) {
	sha, _, err := o.encode(t)
	if err != nil {
		return nil, err
	}
	return sha, nil
}

// Root returns the filesystem root that this *Database works within, if
// backed by a fileStorer (constructed by FromFilesystem). If so, it returns
// the fully-qualified path on a disk and a value of true.
//
// Otherwise, it returns empty-string and a value of false.
func (o *Database) Root() (string, bool) {
	type rooter interface {
		Root() string
	}

	if root, ok := o.rw.(rooter); ok {
		return root.Root(), true
	}
	return "", false
}

// Hasher returns a new hash instance suitable for this object database.
func (o *Database) Hasher() hash.Hash {
	return hasher(o.objectFormat)
}

// encode encodes and saves an object to the storage backend and uses an
// in-memory buffer to calculate the object's encoded body.
func (d *Database) encode(object Object) (sha []byte, n int64, err error) {
	return d.encodeBuffer(object, bytes.NewBuffer(nil))
}

// encodeBuffer encodes and saves an object to the storage backend by using the
// given buffer to calculate and store the object's encoded body.
func (d *Database) encodeBuffer(object Object, buf io.ReadWriter) (sha []byte, n int64, err error) {
	cn, err := object.Encode(buf)
	if err != nil {
		return nil, 0, err
	}

	tmp, err := os.CreateTemp(d.tmp, "")
	if err != nil {
		return nil, 0, err
	}
	defer d.cleanup(tmp)

	to := NewObjectWriter(tmp, d.Hasher())
	if _, err = to.WriteHeader(object.Type(), int64(cn)); err != nil {
		return nil, 0, err
	}

	if seek, ok := buf.(io.Seeker); ok {
		if _, err = seek.Seek(0, io.SeekStart); err != nil {
			return nil, 0, err
		}
	}

	if _, err = io.Copy(to, buf); err != nil {
		return nil, 0, err
	}

	if err = to.Close(); err != nil {
		return nil, 0, err
	}

	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		return nil, 0, err
	}
	return d.save(to.Sha(), tmp)
}

// save writes the given buffer to the location given by the storer "o.s" as
// identified by the sha []byte.
func (o *Database) save(sha []byte, buf io.Reader) ([]byte, int64, error) {
	n, err := o.rw.Store(sha, buf)

	return sha, n, err
}

// open gives an `*ObjectReader` for the given loose object keyed by the given
// "sha" []byte, or an error.
func (o *Database) open(sha []byte) (*ObjectReader, error) {
	if atomic.LoadUint32(&o.closed) == 1 {
		return nil, fmt.Errorf("git/object: cannot use closed *pack.Set")
	}

	f, err := o.ro.Open(sha)
	if err != nil {
		return nil, err
	}
	if o.ro.IsCompressed() {
		return NewObjectReadCloser(f)
	}
	return NewUncompressedObjectReadCloser(f)
}

// openDecode calls decode (see: below) on the object named "sha" after openin
// it.
func (o *Database) openDecode(sha []byte, into Object) error {
	r, err := o.open(sha)
	if err != nil {
		return err
	}
	return o.decode(r, into)
}

// decode decodes an object given by the sha "sha []byte" into the given object
// "into", or returns an error if one was encountered.
//
// Ordinarily, it closes the object's underlying io.ReadCloser (if it implements
// the `io.Closer` interface), but skips this if the "into" Object is of type
// BlobObjectType. Blob's don't exhaust the buffer completely (they instead
// maintain a handle on the blob's contents via an io.LimitedReader) and
// therefore cannot be closed until signaled explicitly by object.Blob.Close().
func (o *Database) decode(r *ObjectReader, into Object) error {
	typ, size, err := r.Header()
	if err != nil {
		return err
	} else if typ != into.Type() {
		return &UnexpectedObjectType{Got: typ, Wanted: into.Type()}
	}

	if _, err = into.Decode(o.Hasher(), r, size); err != nil {
		return err
	}

	if into.Type() == BlobObjectType {
		return nil
	}
	return r.Close()
}

func (o *Database) cleanup(f *os.File) {
	_ = f.Close()
	_ = os.Remove(f.Name())
}

func hasher(algo ObjectFormatAlgorithm) hash.Hash {
	switch algo {
	case ObjectFormatSHA1:
		return sha1.New()
	case ObjectFormatSHA256:
		return sha256.New()
	default:
		return nil
	}
}
