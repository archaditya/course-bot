'use client';

import { useParams } from 'next/navigation';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  apiGetProject, apiListCourses, apiCreateCourse,
  apiCreateConversation, apiUpload, Course,
} from '@/lib/api';
import { Button, Badge, ProcessingStepper, Spinner } from '@/design-system';
import { useState, useCallback } from 'react';
import { useRouter } from 'next/navigation';
import { wsEvents } from '@/lib/ws';
import React from 'react';

function statusBadgeVariant(status: string) {
  if (status === 'INDEXED') return 'success';
  if (status === 'FAILED') return 'error';
  if (status === 'CREATED') return 'default';
  return 'warning';
}

export default function ProjectPage() {
  const { id: projectId } = useParams<{ id: string }>();
  const router = useRouter();
  const queryClient = useQueryClient();
  const [uploadProgress, setUploadProgress] = useState<Record<string, number>>({});
  const [dragging, setDragging] = useState(false);
  const [newCourseTitle, setNewCourseTitle] = useState('');
  const [showNewCourse, setShowNewCourse] = useState(false);

  const { data: project } = useQuery({
    queryKey: ['project', projectId],
    queryFn: () => apiGetProject(projectId),
  });

  const { data: coursesData, isLoading } = useQuery({
    queryKey: ['courses', projectId],
    queryFn: () => apiListCourses(projectId),
  });

  // Listen for indexing status updates via WebSocket
  React.useEffect(() => {
    return wsEvents.on('INDEXED', (payload) => {
      queryClient.invalidateQueries({ queryKey: ['courses', projectId] });
    });
  }, [projectId, queryClient]);

  const { mutate: createCourse, isPending: creatingCourse } = useMutation({
    mutationFn: () => apiCreateCourse(projectId, newCourseTitle),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['courses', projectId] });
      setNewCourseTitle('');
      setShowNewCourse(false);
    },
  });

  const handleUpload = useCallback(async (courseId: string, file: File) => {
    setUploadProgress((p) => ({ ...p, [courseId]: 0 }));
    try {
      await apiUpload(courseId, projectId, file, (pct) => {
        setUploadProgress((p) => ({ ...p, [courseId]: pct }));
      });
      queryClient.invalidateQueries({ queryKey: ['courses', projectId] });
    } catch (err) {
      alert(`Upload failed: ${err instanceof Error ? err.message : 'Unknown error'}`);
    } finally {
      setUploadProgress((p) => { const next = { ...p }; delete next[courseId]; return next; });
    }
  }, [projectId, queryClient]);

  const handleStartChat = async (courseId: string) => {
    const conv = await apiCreateConversation(projectId);
    router.push(`/chats/${conv.id}?course=${courseId}`);
  };

  return (
    <div>
      <div style={{ marginBottom: 'var(--space-8)' }}>
        <h1 style={{ fontFamily: 'var(--font-display)', fontSize: 'var(--text-3xl)', fontWeight: 700 }}>
          {project?.name ?? '…'}
        </h1>
      </div>

      <div style={{
        display: 'flex',
        justifyContent: 'space-between',
        alignItems: 'center',
        marginBottom: 'var(--space-4)',
      }}>
        <h2 style={{ fontFamily: 'var(--font-display)', fontSize: 'var(--text-xl)' }}>Courses</h2>
        <Button id="btn-add-course" size="sm" onClick={() => setShowNewCourse(true)}>
          + Add course
        </Button>
      </div>

      {showNewCourse && (
        <div style={{
          background: 'var(--color-surface)',
          border: '1px solid var(--color-border)',
          borderRadius: 'var(--radius-lg)',
          padding: 'var(--space-4)',
          marginBottom: 'var(--space-4)',
          display: 'flex',
          gap: 'var(--space-3)',
          alignItems: 'flex-end',
        }}>
          <input
            id="course-title"
            value={newCourseTitle}
            onChange={(e) => setNewCourseTitle(e.target.value)}
            placeholder="Course title"
            onKeyDown={(e) => e.key === 'Enter' && newCourseTitle && createCourse()}
            style={{
              flex: 1, padding: '10px 14px', border: '1px solid var(--color-border)',
              borderRadius: 'var(--radius-md)', fontSize: 'var(--text-base)',
              fontFamily: 'var(--font-body)', background: 'var(--color-paper)',
            }}
          />
          <Button id="btn-create-course" onClick={() => createCourse()} disabled={!newCourseTitle} loading={creatingCourse} size="sm">Create</Button>
          <Button variant="ghost" size="sm" onClick={() => setShowNewCourse(false)}>Cancel</Button>
        </div>
      )}

      {isLoading ? (
        <div style={{ display: 'flex', justifyContent: 'center', padding: 'var(--space-8)' }}><Spinner /></div>
      ) : !coursesData?.items?.length ? (
        <div style={{
          textAlign: 'center', padding: 'var(--space-12)',
          background: 'var(--color-surface)', borderRadius: 'var(--radius-2xl)',
          border: '2px dashed var(--color-border)',
        }}>
          <p style={{ fontSize: 'var(--text-4xl)', marginBottom: 'var(--space-3)' }}>📄</p>
          <h3 style={{ fontFamily: 'var(--font-display)', marginBottom: 'var(--space-2)' }}>No courses yet</h3>
          <p style={{ color: 'var(--color-ink-secondary)', marginBottom: 'var(--space-4)' }}>
            Add a course and upload its transcript to start chatting.
          </p>
          <Button id="btn-first-course" onClick={() => setShowNewCourse(true)}>Add your first course</Button>
        </div>
      ) : (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-4)' }}>
          {coursesData.items.map((course: Course) => (
            <CourseCard
              key={course.id}
              course={course}
              uploadProgress={uploadProgress[course.id]}
              onUpload={(file) => handleUpload(course.id, file)}
              onChat={() => handleStartChat(course.id)}
            />
          ))}
        </div>
      )}
    </div>
  );
}

function CourseCard({ course, uploadProgress, onUpload, onChat }: {
  course: Course;
  uploadProgress?: number;
  onUpload: (file: File) => void;
  onChat: () => void;
}) {
  const fileRef = React.useRef<HTMLInputElement>(null);
  const processing = !['INDEXED', 'CREATED', 'FAILED'].includes(course.status);

  return (
    <div style={{
      background: 'var(--color-surface)',
      border: '1px solid var(--color-border-subtle)',
      borderRadius: 'var(--radius-xl)',
      padding: 'var(--space-5)',
      boxShadow: 'var(--shadow-sm)',
    }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: 'var(--space-4)' }}>
        <div>
          <h3 style={{ fontFamily: 'var(--font-display)', fontSize: 'var(--text-lg)', fontWeight: 600, marginBottom: 'var(--space-1)' }}>
            {course.title}
          </h3>
          <Badge variant={statusBadgeVariant(course.status)}>{course.status}</Badge>
        </div>
        <div style={{ display: 'flex', gap: 'var(--space-2)' }}>
          {course.status === 'INDEXED' && (
            <Button id={`btn-chat-${course.id}`} size="sm" onClick={onChat}>Chat</Button>
          )}
          {(course.status === 'CREATED' || course.status === 'FAILED') && (
            <>
              <Button id={`btn-upload-${course.id}`} size="sm" variant="secondary"
                onClick={() => fileRef.current?.click()}>
                Upload course
              </Button>
              <input
                ref={fileRef}
                type="file"
                accept=".srt"
                style={{ display: 'none' }}
                onChange={(e) => e.target.files?.[0] && onUpload(e.target.files[0])}
              />
            </>
          )}
        </div>
      </div>

      {uploadProgress != null && (
        <div style={{
          background: 'var(--color-paper-subtle)',
          borderRadius: 'var(--radius-full)',
          height: '6px',
          overflow: 'hidden',
          marginBottom: 'var(--space-4)',
        }}>
          <div style={{
            height: '100%',
            width: `${uploadProgress}%`,
            background: 'var(--color-accent)',
            borderRadius: 'var(--radius-full)',
            transition: 'width 200ms ease',
          }} />
        </div>
      )}

      {processing && <ProcessingStepper status={course.status} />}
    </div>
  );
}
