import React from 'react';
import { Icon } from '@iconify/react';

interface AnimatedIconProps {
  icon: string;
  width?: number;
  height?: number;
  className?: string;
  style?: React.CSSProperties;
}

const AnimatedIcon: React.FC<AnimatedIconProps> = ({
  icon,
  width = 20,
  height = 20,
  className,
  style,
}) => (
  <Icon
    icon={icon}
    width={width}
    height={height}
    className={className}
    style={{ flexShrink: 0, ...style }}
  />
);

export default AnimatedIcon;
