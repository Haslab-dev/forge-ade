import React from 'react';
import { render, screen, fireEvent, cleanup } from '@testing-library/react';
import { describe, it, expect, vi, afterEach } from 'vitest';
import { RemoteFileUploaderModal } from './RemoteFileUploaderModal';
import { WorkspaceProvider, useWorkspace } from '../../stores/workspaceStore';

const TestWrapper: React.FC<{ isOpen: boolean; onClose: () => void; initialTarget?: string }> = ({
  isOpen,
  onClose,
  initialTarget
}) => {
  const { setRemoteUploadTargetDir } = useWorkspace();

  React.useEffect(() => {
    if (initialTarget) {
      setRemoteUploadTargetDir(initialTarget);
    }
  }, [initialTarget, setRemoteUploadTargetDir]);

  return <RemoteFileUploaderModal isOpen={isOpen} onClose={onClose} />;
};

describe('RemoteFileUploaderModal', () => {
  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
  });

  it('renders nothing when isOpen is false', () => {
    render(
      <WorkspaceProvider>
        <TestWrapper isOpen={false} onClose={vi.fn()} />
      </WorkspaceProvider>
    );

    expect(screen.queryByText('Upload to Remote Server')).not.toBeInTheDocument();
  });

  it('renders modal with remote destination path when isOpen is true', () => {
    const handleClose = vi.fn();
    render(
      <WorkspaceProvider>
        <TestWrapper
          isOpen={true}
          onClose={handleClose}
          initialTarget="ssh://my-conn-1/var/www/site"
        />
      </WorkspaceProvider>
    );

    expect(screen.getByText('Upload to Remote Server')).toBeInTheDocument();
    expect(screen.getByText('SFTP')).toBeInTheDocument();
    expect(screen.getByText('Browse Files...')).toBeInTheDocument();
    expect(screen.getByText('Browse Folder...')).toBeInTheDocument();

    const input = screen.getByPlaceholderText(/ssh:\/\/|var\/www/) as HTMLInputElement;
    expect(input.value).toBe('ssh://my-conn-1/var/www/site');
  });

  it('calls onClose when close button is clicked', () => {
    const handleClose = vi.fn();
    render(
      <WorkspaceProvider>
        <TestWrapper isOpen={true} onClose={handleClose} />
      </WorkspaceProvider>
    );

    const closeBtn = screen.getByTitle('Close (Esc)');
    fireEvent.click(closeBtn);
    expect(handleClose).toHaveBeenCalledTimes(1);
  });
});
