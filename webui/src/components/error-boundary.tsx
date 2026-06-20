import { Component, type ErrorInfo, type ReactNode } from 'react';
import { QueryErrorResetBoundary } from '@tanstack/react-query';
import { AlertCircle, RefreshCw } from 'lucide-react';
import { Button } from '@/components/ui/button';

interface InnerProps {
  onReset: () => void;
  fallback?: ReactNode;
  children: ReactNode;
}

interface InnerState {
  hasError: boolean;
  error: Error | null;
}

class ErrorBoundaryInner extends Component<InnerProps, InnerState> {
  state: InnerState = { hasError: false, error: null };

  static getDerivedStateFromError(error: Error): InnerState {
    return { hasError: true, error };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error('[ErrorBoundary]', error, info.componentStack);
  }

  handleReset = () => {
    this.props.onReset();
    this.setState({ hasError: false, error: null });
  };

  render() {
    if (this.state.hasError) {
      if (this.props.fallback) return this.props.fallback;
      return (
        <div className="flex flex-col items-center justify-center gap-4 p-8 text-center">
          <AlertCircle className="h-10 w-10 text-destructive" />
          <div>
            <h3 className="text-lg font-semibold">页面加载出错</h3>
            <p className="mt-1 text-sm text-muted-foreground">
              {this.state.error?.message ?? '发生了未知错误'}
            </p>
          </div>
          <Button variant="outline" size="sm" onClick={this.handleReset}>
            <RefreshCw className="mr-2 h-4 w-4" />
            重试
          </Button>
        </div>
      );
    }
    return this.props.children;
  }
}

interface ErrorBoundaryProps {
  fallback?: ReactNode;
  children: ReactNode;
}

export function ErrorBoundary({ fallback, children }: ErrorBoundaryProps) {
  return (
    <QueryErrorResetBoundary>
      {({ reset }) => (
        <ErrorBoundaryInner onReset={reset} fallback={fallback}>
          {children}
        </ErrorBoundaryInner>
      )}
    </QueryErrorResetBoundary>
  );
}
