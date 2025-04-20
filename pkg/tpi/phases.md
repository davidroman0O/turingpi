Looking at the document, here's the detailed plan in order of dependencies and impact:

1. **First Phase - Core Utilities** (No/Minimal Dependencies)
```markdown
A. pkg/tpi/platform/ - REUSE
   - Current: OS detection, Docker checks
   - Status: Keep as is, it's a core utility

B. pkg/tpi/context.go - REUSE
   - Current: Context wrapper and Logger interface
   - Status: Keep as is, provides needed functionality

C. pkg/tpi/state/ - REUSE with Minor Updates
   - Current: State persistence (NodeState, SystemState, FileStateManager)
   - Add: LastCacheKeyUsed field to NodeState
   - Purpose: Informational/auditing for TuringPiProvider
```

2. **Second Phase - Infrastructure Packages**
```markdown
A. pkg/tpi/cache/ - CREATE NEW
   - Define:
     ```go
     type Metadata struct {
         Key         string
         Filename    string
         ContentType string
         Size        int64
         ModTime     time.Time
         Hash        string            // Optional SHA256
         Tags        map[string]string // User-defined tags
         OSType      string
         OSVersion   string
     }

     type Cache interface {
         Put(ctx context.Context, key string, metadata Metadata, reader io.Reader) (*Metadata, error)
         Get(ctx context.Context, key string, getContent bool) (*Metadata, io.ReadCloser, error)
         Stat(ctx context.Context, key string) (*Metadata, error)
         Exists(ctx context.Context, key string) (bool, error)
         List(ctx context.Context, filterTags map[string]string) ([]Metadata, error)
         Delete(ctx context.Context, key string) error
         Location() string
     }
     ```
   - Implement: local_file_cache.go and remote_sftp_cache.go
   - Add: GenerateContentHash helper
   - Add: Retry logic for network operations


B. pkg/tpi/node/ - ENHANCE
   - Keep: Core functions (ExecuteCommand, ExpectAndSend)
   - Add: Retry logic for network operations
   - Fix: Close() to properly cleanup per-instance resources
   - Update: Use SSHCache for all file operations
   - Add: BMC-specific command helpers (if needed)
   - Note: This package now handles all remote operations (both BMC and regular SSH)
```

3. **Third Phase - Core Functionality**
```markdown
A. pkg/tpi/docker/ - REFINE
   - Keep: Core adapter (adapter.go, container.go)
   - Remove: Global registry from container_registry.go
   - Change: Container lifecycle tied to DockerAdapter instance
   - Ensure: Proper cleanup on operation completion

B. pkg/tpi/imageops/ - ENHANCE
   - Keep: Core image manipulation logic
   - Improve: Error handling and cleanup in Docker adapter flow
   - Add: Reliable file checksum calculation
   - Verify: Proper PrepImageDir usage via Cluster context

C. Move image_modifier.go to imageops package
   - Add: Value(key string) interface{} method
   - Add: contextMap for dynamic data
   - Add: SetContextValue method
```

4. **Fourth Phase - New OS Package Structure**
```markdown
A. pkg/tpi/os/ - CREATE NEW
   - Create: factory.go
     ```go
     type Provider interface {
         NewImageBuilder(nodeID tpi.NodeID) ImageBuilder
         NewOSInstaller(nodeID tpi.NodeID) OSInstaller
         NewPostInstaller(nodeID tpi.NodeID) PostInstaller
     }

     func RegisterProvider(id OSIdentifier, provider Provider)
     func GetProvider(id OSIdentifier) (Provider, error)
     ```

B. pkg/tpi/os/ubuntu/ - CREATE NEW
   - Implement:
     - ubuntu.Provider
     - ubuntu.ImageBuilder
     - ubuntu.OSInstaller
     - ubuntu.PostInstaller
     - ubuntu.Runtime
   - Define: Config structs with CacheKey
   - Define: Version constants
```

5. **Fifth Phase - Core API Refactor**
```markdown
A. pkg/tpi/types.go - REFACTOR
   - Add: All core toolkit interfaces
   - Add: OSIdentifier struct
   - Update: TPIConfig (add cache dirs, remove per-node OS info)
   - Update: NodeConfig (only IP and Board)
   - Add: ImageResult struct
   - Add: NodePrepareID constant

B. pkg/tpi/tpi.go - HEAVY REFACTOR
   - Rename: TuringPiExecutor to TuringPiProvider
   - Remove: Run(func) workflow method
   - Implement: Cluster interface methods
   - Keep: Helper functions like GetNodeConfig
   - Optional: Add convenience methods
```

6. **Final Phase - Migration**
```markdown
A. Testing with Real Hardware
   - Use BMC at 192.168.1.90 (root/turing)
   - Test with real Ubuntu images
   - Create test configuration file

B. Gradual Migration
   - Keep pkg/tpi/ubuntu/ until new implementation is stable
   - Migrate functionality piece by piece
   - Verify with real hardware tests
   - Remove old package when migration is complete
```

Would you like me to:
1. Start implementing any specific phase?
2. Detail the implementation plan for any specific package?
3. Create the test configuration for real hardware testing?
