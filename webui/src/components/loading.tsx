import React from 'react';

interface LoadingProps {
  message?: string;
  className?: string;
}

const Loading: React.FC<LoadingProps> = ({ message = '加载中', className = '' }) => {
  return (
    <div className={`flex flex-col items-center justify-center ${className}`}>
      <div className="loading-dots">
        <div></div>
        <div></div>
        <div></div>
        <div></div>
      </div>
      <div className="mt-4 text-muted-foreground">
        {message}...
      </div>
    </div>
  );
};

export default Loading; 
