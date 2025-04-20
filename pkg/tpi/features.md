**Comprehensive Feature & Architecture Goals (Revision 7 - User-Keyed Cache)**


**Features:**

**I. Core API & Architecture ("Toolkit" Approach):**

1.  **Service Provider (`TuringPiProvider`):** The main entry point, initializes resources based on `TPIConfig`, provides the `Cluster` interface context. Does *not* orchestrate workflows itself. *(Implementation as discussed previously)*.
2.  **`Cluster` Interface:** Facade providing controlled access to shared services (OS Provider Factory, Caches, BMC Config, State Manager, Configured Paths). *(Definition as discussed previously)*.
3.  **User Composition:** Users explicitly orchestrate provisioning steps in their Go code by obtaining toolkit components (`ImageBuilder`, etc.) via the `Cluster` interface and calling their methods sequentially with specific config structs.

**II. Configuration & Type Safety:**

4.  **Declarative, OS-Specific Configuration:** Users define phase parameters using concrete, type-safe Go structs defined within OS packages (e.g., `ubuntu.ImageBuildConfig`, `ubuntu.InstallConfig`).
5.  **`CacheKey` Requirement:** `ImageBuildConfig` **must** include a user-defined `CacheKey string` field, which serves as the **primary unique identifier** for the prepared image in the remote cache.
    *   **Definition (`pkg/tpi/os/ubuntu/config.go` - Emphasis):**
        ```go
        package ubuntu
        // ... imports ...
        type ImageBuildConfig struct {
            // CacheKey: User-defined UNIQUE key for this specific build config. REQUIRED.
            // Used to identify the image in the remote cache.
            CacheKey               string 
            Version                string 
            BoardType              tpi.BoardType 
            NetworkConfig          tpi.NetworkConfig 
            ImageCustomizationFunc func(image tpi.ImageModifier) error // Optional
            BaseImageXZPath        string // Optional Override for local source image
            Tags                   map[string]string // Optional user-defined tags (key-value) for metadata
            ForceRebuild           bool // Optional: Ignore cache check and always rebuild
        }
        ```
6.  **ACL via `Configure`:** Builder `Configure` methods validate the *specific* incoming config struct type and its required fields (including `CacheKey`). Performs structural and basic validation.
    *   **Implementation Snippet (`pkg/tpi/os/ubuntu/image_builder.go`):**
        ```go
        func (b *ImageBuilder) Configure(config interface{}) error {
            cfg, ok := config.(ImageBuildConfig) // 1. Type Assert
            if !ok { return fmt.Errorf("expected ubuntu.ImageBuildConfig, got %T", config) }
            // 2. Validate required fields (CacheKey, Version, BoardType, NetworkConfig)
            if cfg.CacheKey == "" { return fmt.Errorf("CacheKey is required") }
            if cfg.Version == "" { /* return validation error */ }
            // ... other basic structural validations ...
            b.config = &cfg // 3. Store Validated Config
            return nil
        }
        ```
7.  **OS Version Constants:** Define and use constants like `ubuntu.V2204LTS`.
8.  **Simplified `TPIConfig`/`NodeConfig`:** Global paths/BMC; Static node IP/Board.

**III. Caching & Verification (User-Keyed Focus):**

9.  **Generic Cache Interfaces (`cache.Cache`):** Define standard methods (`Put`, `Get`, `Stat`, `Exists`, `List`, `Delete`) operating on user-provided keys. `cache.Metadata` struct holds associated info, including the `CacheKey` and user `Tags`. *(Definition as discussed previously)*.
10. **Local Base Image Cache & Auto-Download:** Manage source OS images locally (`LocalSourceCacheDir`), accessed via `cluster.GetLocalCache()`. Implement auto-download based on `Version`/`BoardType` if `BaseImageXZPath` isn't set. Uses internal keys (e.g., "baseimg:ubuntu:22.04:rk1").
11. **Remote Prepared Image Cache:** Store prepared images (`.img.xz`) + metadata (`.meta`) on BMC (`RemoteCacheDir`), accessed via `cluster.GetRemoteCache()`. Items are identified/stored using the **user-provided `CacheKey` from the `ImageBuildConfig`**.
12. **Cache Verification (`ImageBuilder.CheckCache`):**
    *   **Role:** Check if an image exists in the remote cache under the user-provided `CacheKey` specified in the configuration.
    *   **Implementation:** Takes the `CacheKey` from the configured `ImageBuildConfig`. Calls `remoteCache.Stat(ctx, cacheKey)`. Returns `(true, ImageResult{ImagePath: cacheKey, CacheKey: cacheKey, IsRemoteCache: true, ...}, nil)` if `Stat` succeeds (key exists). **No automatic content verification against current config.** The user is responsible for ensuring the `CacheKey` corresponds to the desired configuration.
    *   **Interface Definition (`pkg/tpi/types.go`):**
        ```go
         type ImageBuilder interface {
             Configure(config interface{}) error
             // CheckCache checks if an item exists for the configured CacheKey.
             // Returns true and metadata-derived ImageResult if key found in remote cache.
             CheckCache(ctx context.Context, cluster Cluster) (exists bool, result ImageResult, err error)
             Run(ctx context.Context, cluster Cluster) (ImageResult, error)
             // ... other methods like SetModifierContext ...
         }
        ```
13. **Metadata Storage & Tags:** `cache.Metadata` stored in `.meta` files on the BMC, keyed by `CacheKey`. Includes user `Tags`, system info, `CacheKey`, and potentially an optional content `Hash` calculated during `Put`. `ImageBuilder` populates this during the `Put` operation in its `Run` method, using the `CacheKey` and `Tags` from the configuration. Users can query the cache using tags via `cache.List`.

**IV. Image Building & Customization:**

14. **`ImageBuilder.Run`:**
    *   **Role:** Perform the actual build, optionally check cache (using `CacheKey`), and store result in remote cache (using `CacheKey`).
    *   **Implementation:**
        *   Must be called after `Configure`.
        *   Checks `ForceRebuild` flag from config.
        *   Optionally calls `CheckCache(ctx, cluster)` using `b.config.CacheKey`. If hit and not forcing rebuild, returns cached `ImageResult`.
        *   If build proceeds:
            *   Resolves source image (using local cache/download).
            *   Executes build using `imageops` (incl. running `ImageCustomizationFunc`).
            *   Creates `cache.Metadata` (using `b.config.CacheKey`, `b.config.Tags`, calculating optional content `Hash`).
            *   Uploads image + metadata to remote cache using `remoteCache.Put(ctx, b.config.CacheKey, metadata, ...)`.
            *   Returns `ImageResult{ImagePath: localBuiltPath, CacheKey: b.config.CacheKey, IsRemoteCache: false, ...}`.
15. **Image Customization (`ImageCustomizationFunc`):** User function provided in config, executed during `Run`. `ImageModifier` context remains.

**V. OS Installation & Flashing:**

16. **Board-Specific Flashing (`board.Flasher`):** Isolate hardware flashing logic using the `bmc` adapter. Operates on *remote* BMC image paths. *(Implementation as discussed)*.
17. **OS Installation Flow (`OSInstaller.Run`):**
    *   **Role:** Install OS using cached/provided image, handle first boot.
    *   **Implementation:** Accepts `tpi.ImageResult`. Checks `IsRemoteCache`. If true, uses `remoteCache.Get(ctx, imageResult.CacheKey, true)` to stream content from remote cache (identified by `CacheKey`) via BMC adapter to a temporary BMC path. If false, uploads local `imageResult.ImagePath` via BMC adapter to a temporary BMC path. Calls `flasher.Flash` with the temporary BMC path. Performs internal OS post-install.
18. **Internal OS Post-Install:** Part of `OSInstaller.Run` *after* flashing. Uses `node` adapter and `InstallConfig.TargetPassword`.

**VI. Post-Installation:**

19. **User Post-Install Actions (`PostActions`):** Allow users to provide a Go function to execute arbitrary commands/tasks on the newly provisioned node via SSH.
20. **Runtime Interfaces (`LocalRuntime`, `OSRuntime`):** Provide abstracted access to local machine and remote node functionalities within the `PostActions` function. Connection uses credentials from `PostInstallConfig`.

**VII. Operational Aspects:**

21. **Temporary Processing Folder (`PrepImageDir`):** Used by `imageops` for intermediate files.
22. **Node Agnostic ID (`tpi.NodePrepareID`):** Used for prepare-only actions.
23. **Logging:** Structured logging, per-operation local log files.
24. **Error Handling & Retries:** Retries for network ops in `bmc`/`node`. Clear errors. Disk space checks.
25. **Cleanup:** Manage `PrepImageDir`. (Cache pruning TBD).
26. **State Management:** `StateManager` for informational logging/auditing.


**I. Core API & Architecture ("Toolkit" Approach):**

1.  **Service Provider (`TuringPiProvider`):**
    *   **Role:** The main entry point. Initializes the library based on `TPIConfig`, sets up access to shared resources (BMC, State Manager, Caches), and provides the `Cluster` interface context. It does *not* run workflows itself.
    *   **Implementation Snippet (`pkg/tpi/tpi.go` - Conceptual):**
        ```go
        package tpi

        import (
        	"github.com/davidroman0O/turingpi/pkg/tpi/bmc"
        	"github.com/davidroman0O/turingpi/pkg/tpi/cache"
        	"github.com/davidroman0O/turingpi/pkg/tpi/os"
        	"github.com/davidroman0O/turingpi/pkg/tpi/state"
        	// ... other imports
        )

        // TuringPiProvider holds initialized services and configuration.
        type TuringPiProvider struct {
        	config        TPIConfig
        	stateManager  state.Manager
        	localCache    cache.Cache
        	remoteCache   cache.Cache
        	// other fields like bmc adapter factory if needed
        }

        // NewTuringPiProvider initializes all necessary components.
        func NewTuringPiProvider(cfg TPIConfig) (*TuringPiProvider, error) {
        	// 1. Validate cfg (paths, BMC creds)
        	// 2. Ensure cache/prep directories exist
        	// 3. Initialize State Manager
        	stateMgr, err := state.NewFileStateManager(filepath.Join(cfg.CacheDir, "tpi_state.json"))
        	if err != nil { /* handle */ }
        	// 4. Initialize Local Cache Implementation
        	localCacheImpl, err := cache.NewLocalFileCache(cfg.LocalSourceCacheDir)
        	if err != nil { /* handle */ }
        	// 5. Initialize Remote Cache Implementation (needs BMC adapter)
        	bmcConf := bmc.SSHConfig{ /* from cfg */ }
        	bmcAdapter := bmc.NewBMCAdapter(bmcConf) // Create BMC adapter
        	remoteCacheImpl, err := cache.NewRemoteSftpCache(cfg.RemoteCacheDir, bmcAdapter)
        	if err != nil { /* handle */ }

        	provider := &TuringPiProvider{
        		config:        cfg,
        		stateManager:  stateMgr,
        		localCache:    localCacheImpl,
        		remoteCache:   remoteCacheImpl,
        	}
        	return provider, nil
        }

        // --- Methods to satisfy tpi.Cluster interface ---

        func (p *TuringPiProvider) GetStateManager() state.Manager { return p.stateManager }
        func (p *TuringPiProvider) GetProvider(id OSIdentifier) (os.Provider, error) { return os.GetProvider(id) }
        func (p *TuringPiProvider) GetBMCSSHConfig() bmc.SSHConfig { /* create from p.config */ }
        func (p *TuringPiProvider) GetLocalCache() cache.Cache { return p.localCache }
        func (p *TuringPiProvider) GetRemoteCache() cache.Cache { return p.remoteCache }
        func (p *TuringPiProvider) GetCacheDir() string { return p.config.CacheDir }
        func (p *TuringPiProvider) GetPrepImageDir() string { return p.config.PrepImageDir }

        // Helper needed by toolkit users
        func (p *TuringPiProvider) GetNodeConfig(id NodeID) *NodeConfig { /* return from p.config */ }

        ```

2.  **`Cluster` Interface:**
    *   **Role:** A facade provided by `TuringPiProvider` giving controlled access to services needed by user workflows and internal builders.
    *   **Definition (`pkg/tpi/types.go`):**
        ```go
        package tpi

        import (
        	"github.com/davidroman0O/turingpi/pkg/tpi/bmc"
        	"github.com/davidroman0O/turingpi/pkg/tpi/cache"
        	"github.com/davidroman0O/turingpi/pkg/tpi/os"
        	"github.com/davidroman0O/turingpi/pkg/tpi/state"
        )

        type Cluster interface {
        	GetStateManager() state.Manager
        	GetProvider(id OSIdentifier) (os.Provider, error) // Gets OS-specific tool factory
        	GetBMCSSHConfig() bmc.SSHConfig

        	// Cache Access
        	GetLocalCache() cache.Cache
        	GetRemoteCache() cache.Cache

        	// Directory Paths (for non-cache operations)
        	GetCacheDir() string      // Base dir for local state/log files
        	GetPrepImageDir() string  // Base dir for image processing temp files
        }
        ```

3.  **User Composition:**
    *   **Role:** The user's Go code orchestrates the provisioning by obtaining tools from the `Cluster` context and calling their methods sequentially.
    *   **Example Snippet (User Workflow):**
        ```go
        // provider is the *TuringPiProvider instance
        ctx := context.Background()
        nodeID := tpi.Node1
        targetOS := tpi.OSIdentifier{Type: "ubuntu", Version: ubuntu.V2204LTS}

        osProvider, err := provider.GetProvider(targetOS)
        if err != nil { /* handle */ }

        imgBuilder := osProvider.NewImageBuilder(nodeID)
        // User defines the CacheKey explicitly
        imgCfg := ubuntu.ImageBuildConfig{ CacheKey: "ubuntu-2204-rk1-base-net", /* ... other fields ... */ }
        err = imgBuilder.Configure(imgCfg)
        if err != nil { /* handle */ }

        exists, cachedResult, err := imgBuilder.CheckCache(ctx, provider) // Pass provider as Cluster
        if err != nil { /* handle cache check error */ }

        var imageResult tpi.ImageResult
        if exists && !imgCfg.ForceRebuild {
             log.Printf("Using cached image from remote cache (Key: %s)", imgCfg.CacheKey)
             imageResult = cachedResult
        } else {
             log.Printf("Building image (Key: %s)...", imgCfg.CacheKey)
             imageResult, err = imgBuilder.Run(ctx, provider) // Build if needed
             if err != nil { /* handle build error */ }
        }
        // ... use imageResult ...

        installer := osProvider.NewOSInstaller(nodeID)
        installCfg := ubuntu.InstallConfig{ /* ... */ }
        err = installer.Configure(installCfg)
        // ... configure and run installer using imageResult ...
        ```

**II. Configuration & Type Safety:**

4.  **OS-Specific Config Structs:**
    *   **Role:** Define parameters for each OS/phase in a type-safe manner. Created by the user. Includes the required `CacheKey`.
    *   **Definition (`pkg/tpi/os/ubuntu/config.go` - Example):**
        ```go
        package ubuntu
        // ... imports ...
        const V2204LTS = "22.04"
        type ImageBuildConfig struct {
            CacheKey               string `validation:"required"` // User-provided key
            Version                string `validation:"required"`
            BoardType              tpi.BoardType `validation:"required"`
            NetworkConfig          tpi.NetworkConfig `validation:"required"`
            ImageCustomizationFunc func(image tpi.ImageModifier) error
            BaseImageXZPath        string // Optional Override
            Tags                   map[string]string // Changed to map
            ForceRebuild           bool
        }
        type InstallConfig struct { TargetPassword string `validation:"required"` /*...*/ }
        type PostInstallConfig struct { Username string `validation:"required"`; Password string `validation:"required"`}
        ```

5.  **ACL via `Configure`:**
    *   **Role:** Validate incoming OS-specific config structs within builders, ensuring required fields like `CacheKey` are present.
    *   **Implementation Snippet (`pkg/tpi/os/ubuntu/image_builder.go`):**
        ```go
        func (b *ImageBuilder) Configure(config interface{}) error {
            cfg, ok := config.(ImageBuildConfig) // Assert type
            if !ok { /* return error */ }
            if cfg.CacheKey == "" { return fmt.Errorf("CacheKey is required") } // Validate CacheKey
            if cfg.Version == "" { /* return validation error */ }
            if cfg.BoardType == "" { /* return validation error */ }
            // ... validate NetworkConfig, potentially BaseImageXZPath existence ...
            b.config = &cfg // Store validated config
            return nil
        }
        ```

6.  **OS Version Constants:**
    *   **Role:** Provide standard identifiers for supported OS versions.
    *   **Definition (`pkg/tpi/os/ubuntu/config.go`):**
        ```go
        package ubuntu
        const V2204LTS = "22.04"
        // const V2404LTS = "24.04"
        ```
    *   **Usage:** `imgCfg := ubuntu.ImageBuildConfig{ CacheKey: "mykey", Version: ubuntu.V2204LTS, ... }`

**III. Caching, Download & Verification Features:**

7.  **Generic Cache Interfaces & Metadata:**
    *   **Role:** Define standard methods (`Put`, `Get`, `Stat`, `Exists`, `List`, `Delete`) for interacting with different cache types (local disk, remote SFTP). Define flexible `Metadata` struct including `Key` and `Tags`.
    *   **Definition (`pkg/tpi/cache/interfaces.go`):**
        ```go
        package cache
        // ... imports ...
        type Metadata struct {
        	Key         string            `json:"key"` // The user-provided cache key
        	Filename    string            `json:"filename,omitempty"`
        	ContentType string            `json:"contentType,omitempty"`
        	Size        int64             `json:"size"`
        	ModTime     time.Time         `json:"modTime"`
        	Hash        string            `json:"hash,omitempty"` // Optional SHA256 content hash
        	Tags        map[string]string `json:"tags,omitempty"` // User-defined tags
        	// OS Info specifically for OS Images
        	OSType    string `json:"osType,omitempty"`
        	OSVersion string `json:"osVersion,omitempty"`
        }
        type Cache interface {
        	Put(ctx context.Context, key string, metadata Metadata, reader io.Reader) (*Metadata, error)
        	Get(ctx context.Context, key string, getContent bool) (*Metadata, io.ReadCloser, error)
        	Stat(ctx context.Context, key string) (*Metadata, error)
        	Exists(ctx context.Context, key string) (bool, error)
        	// List retrieves metadata for items matching all filterTags.
        	List(ctx context.Context, filterTags map[string]string) ([]Metadata, error)
        	Delete(ctx context.Context, key string) error
        	Location() string
        }
        var ErrNotFound = errors.New("cache: item not found")
        // Function to hash file content, useful for populating Metadata.Hash
        func GenerateContentHash(reader io.Reader) (string, error) { /* SHA256 hash */ }
        ```

8.  **Local Base Image Cache & Auto-Download:**
    *   **Role:** Manage base OS images locally, download if needed.
    *   **Implementation Detail:** An internal function `resolveSourceImagePath` used by `ImageBuilder` will interact with the `cache.Cache` instance returned by `cluster.GetLocalCache()`:
        ```go
        // Simplified concept within ImageBuilder
        func (b *ImageBuilder) resolveSource(ctx context.Context, cluster tpi.Cluster) (string, string, error) { // returns path, hash
            localCache := cluster.GetLocalCache()
            if b.config.BaseImageXZPath != "" { // Handle override
                 // Calculate hash of override file, return path & hash
                 // ... hashFile(b.config.BaseImageXZPath) ...
                 return b.config.BaseImageXZPath, calculatedHash, nil
            }
            // Lookup by Version/Board using an internal cache key format
            internalCacheKey := fmt.Sprintf("baseimg:%s:%s:%s", "ubuntu", b.config.Version, b.config.BoardType)
            meta, err := localCache.Stat(ctx, internalCacheKey)
            if err == cache.ErrNotFound {
                log.Printf("Base image %s not in local cache, downloading...", internalCacheKey)
                // url := lookupDownloadURL(b.config.Version, b.config.BoardType) // Find URL
                // httpReader, httpSize, httpErr := downloadHTTP(url) // Download
                // downloadMeta := cache.Metadata{ /* populate OS info, etc. */ }
                // meta, err = localCache.Put(ctx, internalCacheKey, downloadMeta, httpReader) // Store in cache
                // if err != nil { return "", "", err }
                // log.Printf("Download complete. Stored as %s", meta.Key)
                return "", "", fmt.Errorf("auto-download not implemented yet") // Placeholder
            } else if err != nil {
                 return "", "", fmt.Errorf("local cache stat failed: %w", err)
            }
            // Found in cache, return path (derived from key/location) and stored hash
             filePath := filepath.Join(localCache.Location(), internalCacheKey) // Simplistic path derivation
             log.Printf("Using base image from local cache: %s", filePath)
            return filePath, meta.Hash, nil // Assuming meta.Hash contains content hash
        }
        ```

9.  **Remote Prepared Image Cache:**
    *   **Role:** Store prepared images+metadata on BMC.
    *   **Implementation:** `remoteSftpCache` implements `cache.Cache` using `bmc` adapter for SFTP operations on `RemoteCacheDir`. `ImageBuilder.Run` calls `remoteCache.Put` using the user-provided `CacheKey`.

10. **Cache Verification:**
    *   **Role:** Check remote cache for an image matching the **user-provided `CacheKey`**.
    *   **Implementation (`ImageBuilder.CheckCache`):**
        ```go
        func (b *ImageBuilder) CheckCache(ctx context.Context, cluster tpi.Cluster) (bool, tpi.ImageResult, error) {
            if b.config == nil || b.config.CacheKey == "" {
                 return false, tpi.ImageResult{}, fmt.Errorf("cannot check cache without configured CacheKey")
             }
            remoteCache := cluster.GetRemoteCache()
            userCacheKey := b.config.CacheKey // Use the key from the config

            metadata, err := remoteCache.Stat(ctx, userCacheKey)
            if err == cache.ErrNotFound { return false, tpi.ImageResult{}, nil }
            if err != nil { return false, tpi.ImageResult{}, fmt.Errorf("remote cache stat error: %w", err)}

            log.Printf("Cache HIT for key %s", userCacheKey)
            return true, tpi.ImageResult{ // Convert metadata to ImageResult
                 ImagePath:     userCacheKey, // Use key for remote ID when cached
                 CacheKey:      userCacheKey,
                 IsRemoteCache: true,
                 Board:         tpi.BoardType(metadata.Tags["board"]), // Extract info
                 Tags:          metadata.Tags,
                 // ...
            }, nil
        }
        ```

11. **Metadata Storage & Tags:**
    *   **Role:** Store searchable info with cached images.
    *   **Implementation:** `remoteSftpCache.Put` writes the `cache.Metadata` struct (populated by `ImageBuilder` including OS info and user `Tags` from the config) to a `.meta` JSON file on the BMC, associated with the `CacheKey`. Users can list/filter cache items using `cache.List(ctx, filterTags)`.

**IV. OS Installation & Flashing:**

12. **Board-Specific Flashing:**
    *   **Role:** Isolate hardware flashing.
    *   **Implementation (`pkg/tpi/board/rk1_flasher.go`):** *(Remains the same conceptually)*

13. **OS Installation Flow:**
    *   **Role:** Install OS using cached/provided image, handle first boot.
    *   **Implementation (`OSInstaller.Run`):** Accepts `tpi.ImageResult`. Checks `IsRemoteCache`. If true, uses `remoteCache.Get(ctx, imageResult.CacheKey, true)` to stream content from remote cache (identified by **`imageResult.CacheKey`**) via BMC adapter to a temporary BMC path. If false, uploads local `imageResult.ImagePath` via BMC adapter to a temporary BMC path. Calls `flasher.Flash` with the temp path, perform internal post-install.

14. **Internal Post-Install:**
    *   **Role:** Handle OS-specific first-boot tasks (e.g., Ubuntu password change).
    *   **Implementation:** Part of `OSInstaller.Run` *after* `flasher.Flash`. Uses `node` adapter and `InstallConfig.TargetPassword`.

**V. User Post-Installation:**

15. **User Post-Install Phase:**
    *   **Role:** Allow users to run custom commands on the provisioned node.
    *   **Implementation (`PostInstaller.Run`):** Connects via SSH using `PostInstallConfig` credentials, executes `PostActions` func, providing `LocalRuntime` and `OSRuntime`.

16. **Runtimes:**
    *   **Role:** Provide interfaces for interaction during `PostActions`.
    *   **Implementation:** `LocalRuntime` (local file ops), `ubuntu.Runtime` (implements `OSRuntime` using `node` adapter).

**VI. Operational Aspects:**

17. **Temporary Processing Folder (`PrepImageDir`):** Used by `imageops` during `ImageBuilder.Run` for decompression, mounting etc. Cleaned up based on `WithKeepTempOnFailure`.
18. **Node Agnostic ID (`tpi.NodePrepareID`):** Used when calling `provider.NewImageBuilder` for prepare-only actions.
19. **Logging Strategy:** Implement structured logging to per-operation files in local `CacheDir`.
20. **Error Handling & Retries:** Implement retries in `bmc` and `node` adapters for SSH/SFTP operations. Check disk space before large operations.
21. **Cleanup Procedures:** Manage `PrepImageDir` cleanup. `cache.Prune` TBD.
22. **State Management:** `StateManager` records outcomes, last used `CacheKey`, etc. for auditing.
23. **Concurrency Support:** Primarily handled by user (running multiple toolkit workflows). Internal caches need concurrent-safe implementations (e.g., mutexes in `localFileCache`, careful SFTP session handling in `remoteSftpCache`).

**VII. Existing Package Plan (Detailed Roles & Future Status):**


*   **`pkg/tpi/tpi.go`:**
    *   **Current Role:** Executor and main orchestrator (`TuringPiExecutor`, `Run(func)`).
    *   **Future Status:** **Refactor Heavily.**
        *   Rename `TuringPiExecutor` to `TuringPiProvider`.
        *   Remove the high-level `Run(func)` workflow method.
        *   Implement the `tpi.Cluster` interface methods on `TuringPiProvider` to provide access to configuration and initialized services (caches, state manager, OS factory via `GetProvider`, BMC config).
        *   Retain helper functions like `GetNodeConfig(NodeID)`.
        *   Potentially add *new* convenience methods like `PrepareImage(ctx, ImagePrepareWorkflow)` and `ExecuteInstallFromCache(ctx, InstallFromCacheWorkflow)` which internally use the toolkit components (optional additions, not the core API).
        *   **Code Snippet (Conceptual `TuringPiProvider`):**
            ```go
            package tpi
            // ... imports ...
            type TuringPiProvider struct { /* config, stateMgr, localCache, remoteCache */ }
            func NewTuringPiProvider(cfg TPIConfig) (*TuringPiProvider, error) { /* init services */ }
            // Implement tpi.Cluster interface methods...
            func (p *TuringPiProvider) GetProvider(id OSIdentifier) (os.Provider, error) { return os.GetProvider(id) }
            func (p *TuringPiProvider) GetLocalCache() cache.Cache { return p.localCache }
            // ... etc ...
            func (p *TuringPiProvider) GetNodeConfig(id NodeID) *NodeConfig { /* ... */ }
            ```

*   **`pkg/tpi/types.go`:**
    *   **Current Role:** Defines core types (`TPIConfig`, `NodeConfig`, interfaces).
    *   **Future Status:** **Refactor & Expand.**
        *   Define all core toolkit interfaces: `Cluster`, `ImageBuilder`, `OSInstaller`, `PostInstaller`, `OSRuntime`, `ImageModifier`, `LocalRuntime`.
        *   Define `OSIdentifier` struct.
        *   Update `TPIConfig` to include `LocalSourceCacheDir`, `RemoteCacheDir` paths; remove per-node OS info.
        *   Update `NodeConfig` to *only* include static info like `IP`, `Board`.
        *   Define `ImageResult` struct (with `ImagePath`, `CacheKey`, `IsRemoteCache`, `Board`, `Tags`, `LogFilePath`). Remove `ManifestHash`.
        *   Define `NodePrepareID` constant.

*   **`pkg/tpi/context.go`:**
    *   **Current Role:** Defines TPI context wrapper (`tpiContext`) and `Logger` interface.
    *   **Future Status:** **Reuse.** Likely stable, provides necessary context propagation and logging abstraction.

*   **`pkg/tpi/image_modifier.go`:**
    *   **Current Role:** Defines `ImageModifier` interface and `imageModifierImpl` for staging file ops.
    *   **Future Status:** **Enhance.**
        *   Add `Value(key string) interface{}` method to the interface.
        *   Add internal `contextMap map[string]interface{}` to `imageModifierImpl`.
        *   Add an internal method `SetContextValue(key string, value interface{})` used by builders to inject dynamic data (like hostname) before the user's customization func is called.
        *   Update `NewImageModifierImpl` to initialize the map.
        *   **Code Snippet:**
            ```go
            // pkg/tpi/image_modifier.go
            type imageModifierImpl struct {
                 operations []imageops.FileOperation
                 contextMap map[string]interface{} // Added
            }
            func (m *imageModifierImpl) Value(key string) interface{} { /* read from map */ }
            // Internal method for builder:
            func (m *imageModifierImpl) SetContextValue(key string, value interface{}) { /* write to map */ }
            func NewImageModifierImpl() *imageModifierImpl { /* init map */ }
            ```

*   **`pkg/tpi/local_runtime.go`:**
    *   **Current Role:** Defines `LocalRuntime` interface and implementation (`localRuntimeImpl`).
    *   **Future Status:** **Reuse.** Implementation for local file/command execution seems stable.

*   **`pkg/tpi/ubuntu/` (OLD Directory):**
    *   **Current Role:** Contains initial Ubuntu-specific implementation (builders, runtime).
    *   **Future Status:** **DELETE.** Replaced by `pkg/tpi/os/ubuntu/`.

*   **`pkg/tpi/os/` (NEW Directory):**
    *   **Current Role:** N/A.
    *   **Future Status:** **Create.** Parent package for OS abstractions and implementations.
        *   **`factory.go` (NEW):** Implement OS Factory/Registry (`RegisterProvider`, `GetProvider`), defines `os.Provider` interface.
        *   **`ubuntu/` (NEW):** Implement `ubuntu.Provider`, `ubuntu.ImageBuilder`, `ubuntu.OSInstaller`, `ubuntu.PostInstaller`, `ubuntu.Runtime`. Define `ubuntu.*Config` structs (including `CacheKey`) and `ubuntu.Vxxxx` constants. This is where the core OS logic goes.
        *   **`debian/`, etc. (Future):** Add new OS support here following the pattern.

*   **`pkg/tpi/board/` (NEW Directory):**
    *   **Current Role:** N/A.
    *   **Future Status:** **Create.** Define `Flasher` interface and `GetFlasher(BoardType)` function. Implement `RK1Flasher` using the `bmc` adapter. Add other board flashers later.
        *   **Code Snippet (`board.go`):**
            ```go
            package board
            // ... imports ...
            type Flasher interface {
                Flash(ctx context.Context, cluster tpi.Cluster, nodeID tpi.NodeID, remoteBMCImagePath string) error
            }
            func GetFlasher(boardType tpi.BoardType) (Flasher, error) { /* switch based on boardType */ }
            ```

*   **`pkg/tpi/imageops/`:**
    *   **Current Role:** Handles platform-agnostic image manipulations (mount, kpartx, xz, file ops) using native tools or the Docker adapter.
    *   **Future Status:** **Reuse/Enhance.** Core logic is essential. Ensure robustness, especially error handling and cleanup within the Docker adapter flow. Verify it correctly uses the `PrepImageDir` provided via the `Cluster` context.
        *   **Enhancement:** Add function to calculate checksum of a local file reliably.

*   **`pkg/tpi/docker/`:**
    *   **Current Role:** Docker interaction backend for `imageops`. Manages container lifecycle. Includes global container registry.
    *   **Future Status:** **Reuse/Refine.** Core adapter (`adapter.go`, `container.go`) remains necessary for non-Linux `imageops`. **Critically review `container_registry.go`:** Eliminate the global registry and process-wide signal handling. Tie container cleanup strictly to the lifecycle of the `DockerAdapter` instance created and managed *within* `imageops`. When the `imageops` operation using Docker completes (success or failure), its adapter should be explicitly closed, triggering the cleanup of *its specific container*. This avoids global state and ensures cleanup happens correctly even when used as a library.

*   **`pkg/tpi/cache/` (NEW Directory):**
    *   **Current Role:** N/A
    *   **Future Status:** **Create.** Define generic `Cache` interface and `Metadata` struct. Implement `local_file_cache.go` and `remote_sftp_cache.go`. Provide helpers like `GenerateContentHash`. Remove `GenerateManifestHashKey`.

*   **`pkg/tpi/bmc/`:**
    *   **Current Role:** SSH/SFTP interaction with the BMC (`ExecuteCommand`, `CheckFileExists`, `UploadFile`).
    *   **Future Status:** **Enhance.**
        *   Keep existing core functions.
        *   **Add:** `ReadRemoteFile(remotePath string) ([]byte, error)`
        *   **Add:** `WriteRemoteFile(remotePath string, data []byte, perm os.FileMode) error`
        *   **Add:** `MkdirAllRemote(remotePath string) error`
        *   **Add:** `RemoveRemote(remotePath string) error` (for files or potentially empty dirs)
        *   **Consider:** `UploadStream(remotePath string, reader io.Reader, size int64) error` for potentially more efficient large file uploads compared to reading all into memory first for `WriteRemoteFile`.
        *   **Integrate Retries:** Add basic retry logic (e.g., using `go-retry`) around SSH dialing and potentially SFTP operations that might face transient network issues.

*   **`pkg/tpi/node/`:**
    *   **Current Role:** SSH/SFTP interaction with provisioned compute nodes (`ExecuteCommand`, `ExpectAndSend`, `CopyFile`).
    *   **Future Status:** **Reuse/Enhance.**
        *   Keep core functions.
        *   **Integrate Retries:** Add retry logic for SSH dialing and potentially command execution/SFTP operations.
        *   Ensure `Close()` method correctly cleans up all tracked clients/sessions/sftp connections associated *with that specific adapter instance*.

*   **`pkg/tpi/state/`:**
    *   **Current Role:** State persistence (`NodeState`, `SystemState`, `FileStateManager`).
    *   **Future Status:** **Reuse.** Core structures likely sufficient. Update `NodeState` fields if necessary to log relevant info like `LastCacheKeyUsed`. Primarily used for informational/auditing purposes by the `TuringPiProvider`.

*   **`pkg/tpi/platform/`:**
    *   **Current Role:** OS detection, Docker checks.
    *   **Future Status:** **Reuse.** Core utility.
