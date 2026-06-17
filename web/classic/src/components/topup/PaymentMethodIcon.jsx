/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

import React from 'react';
import { CreditCard } from 'lucide-react';
import { SiAlipay, SiStripe, SiWechat } from 'react-icons/si';
import * as SiIcons from 'react-icons/si';
import * as LuIcons from 'react-icons/lu';

const ICON_COMPONENTS = {
  ...SiIcons,
  ...LuIcons,
};

const DEFAULT_ICON_NAMES = {
  alipay: 'SiAlipay',
  huifu: 'LuWalletCards',
  stripe: 'SiStripe',
  waffo_pancake: 'LuCreditCard',
  wxpay: 'SiWechat',
};

function getIconName(value) {
  const trimmed = value?.trim();
  if (!trimmed || !/^[A-Z][A-Za-z0-9]*$/.test(trimmed)) {
    return '';
  }
  return trimmed;
}

function isImageSource(value) {
  const trimmed = value?.trim();
  if (!trimmed) return false;

  return (
    trimmed.startsWith('data:') ||
    trimmed.startsWith('blob:') ||
    trimmed.startsWith('http://') ||
    trimmed.startsWith('https://') ||
    trimmed.startsWith('/') ||
    /\.(svg|png|jpe?g|gif|webp)(\?|#|$)/i.test(trimmed)
  );
}

export default function PaymentMethodIcon({
  payMethod,
  size = 16,
  className,
}) {
  if (!payMethod) {
    return (
      <CreditCard
        className={className}
        size={size}
        color='var(--semi-color-text-2)'
      />
    );
  }

  if (payMethod.type === 'alipay') {
    return <SiAlipay className={className} size={size} color='#1677FF' />;
  }

  if (payMethod.type === 'wxpay') {
    return <SiWechat className={className} size={size} color='#07C160' />;
  }

  if (payMethod.type === 'stripe') {
    return <SiStripe className={className} size={size} color='#635BFF' />;
  }

  const iconValue = payMethod.icon || DEFAULT_ICON_NAMES[payMethod.type] || '';
  const iconName = getIconName(iconValue);
  if (iconName && ICON_COMPONENTS[iconName]) {
    const Icon = ICON_COMPONENTS[iconName];
    return (
      <Icon
        className={className}
        size={size}
        color={payMethod.color || undefined}
      />
    );
  }

  if (isImageSource(iconValue)) {
    return (
      <img
        src={iconValue}
        alt={payMethod.name}
        className={className}
        style={{
          width: size,
          height: size,
          objectFit: 'contain',
        }}
      />
    );
  }

  return (
    <CreditCard
      className={className}
      size={size}
      color={payMethod.color || 'var(--semi-color-text-2)'}
    />
  );
}
