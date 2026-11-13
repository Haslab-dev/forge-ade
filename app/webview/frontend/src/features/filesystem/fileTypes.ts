// File size and binary guards for the code editor and side preview

export const MAX_EDITOR_FILE_BYTES = 8 * 1024 * 1024; // 8MB limit for CodeMirror buffer
export const MAX_SIDE_PREVIEW_TEXT_BYTES = 2 * 1024 * 1024; // 2MB guard for side preview

export const BINARY_EXTENSIONS = new Set([
  // Executables & shared libraries
  'exe', 'dll', 'so', 'dylib', 'bin', 'com', 'elf', 'msi', 'msix', 'appimage',
  'flatpak', 'deb', 'rpm', 'apk', 'ipa', 'pkg', 'app', 'run', 'node',
  // Objects & static libs
  'o', 'obj', 'a', 'lib', 'ko', 'out', 'pdb', 'rlib',
  // Bytecode
  'class', 'jar', 'war', 'ear', 'pyc', 'pyo', 'pyd', 'wasm', 'dex',
  // Archives & disk images
  'zip', 'tar', 'gz', 'tgz', 'bz2', 'xz', 'zst', 'lz4', '7z', 'rar',
  'iso', 'dmg', 'img', 'vhd', 'cab', 'arj', 'lha',
  // Databases (unsupported engines only; sqlite, duckdb, rocksdb have dedicated in-app viewer)
  'sqlite-wal', 'sqlite-shm', 'mdb', 'accdb',
  // Fonts
  'ttf', 'otf', 'woff', 'woff2', 'eot', 'pfb', 'pfm', 'bdf',
  // Media without an in-app viewer
  'mp4', 'm4v', 'mov', 'avi', 'mkv', 'webm', 'flv', 'wmv',
  'mp3', 'wav', 'flac', 'ogg', 'aac', 'm4a', 'opus', 'mid', 'midi',
  // Design/office (zip-based binaries)
  'psd', 'ai', 'sketch', 'fig', 'xd', 'xcf',
  'doc', 'docx', 'xls', 'xlsx', 'ppt', 'pptx', 'pages', 'key', 'numbers', 'odt', 'ods', 'odp',
  // Game/engine assets & misc compiled
  'unity', 'asset', 'uasset', 'pak', 'wad', 'blender', 'blend', '3ds', 'obj-model'
]);

export const ZIP_EXTENSIONS = new Set([
  'zip', 'tar', 'gz', 'tgz', 'bz2', 'xz', 'zst', 'lz4', '7z', 'rar',
  'iso', 'dmg', 'img', 'vhd', 'cab', 'arj', 'lha', 'war', 'ear', 'jar'
]);

export const isZipFile = (path: string): boolean => {
  const name = path.split('/').pop() || path;
  const dot = name.lastIndexOf('.');
  if (dot <= 0) return false;
  return ZIP_EXTENSIONS.has(name.slice(dot + 1).toLowerCase());
};

export const IMAGE_EXTENSIONS = new Set([
  'png', 'jpg', 'jpeg', 'gif', 'webp', 'bmp', 'ico', 'svg', 'avif', 'tiff', 'tif'
]);

export const isImageFile = (path: string): boolean => {
  const name = path.split('/').pop() || path;
  const dot = name.lastIndexOf('.');
  if (dot <= 0) return false;
  return IMAGE_EXTENSIONS.has(name.slice(dot + 1).toLowerCase());
};

export const isDatabaseFile = (path: string): boolean => {
  const name = path.split('/').pop() || path;
  const dot = name.lastIndexOf('.');
  if (dot <= 0) return false;
  const ext = name.slice(dot + 1).toLowerCase();
  return ['sqlite', 'sqlite3', 'db', 'db3', 'duckdb', 'rocksdb', 'ldb', 'sst'].includes(ext);
};

export const isBinaryFile = (path: string): boolean => {
  const name = path.split('/').pop() || path;
  const dot = name.lastIndexOf('.');
  if (dot <= 0) return false;
  const ext = name.slice(dot + 1).toLowerCase();
  if (IMAGE_EXTENSIONS.has(ext)) return false;
  return BINARY_EXTENSIONS.has(ext);
};
