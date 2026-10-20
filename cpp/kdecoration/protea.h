/*
 * SPDX-FileCopyrightText: 2014 Hugo Pereira Da Costa <hugo.pereira@free.fr>
 *
 * SPDX-License-Identifier: GPL-2.0-only OR GPL-3.0-only OR LicenseRef-KDE-Accepted-GPL
 */

#pragma once

#include "proteasettings.h"

#include <QColor>
#include <QList>
#include <QSharedPointer>

namespace Protea
{
//* convenience typedefs
using InternalSettingsPtr = QSharedPointer<InternalSettings>;
using InternalSettingsList = QList<InternalSettingsPtr>;
using InternalSettingsListIterator = QListIterator<InternalSettingsPtr>;

//* metrics
namespace Metrics
{
//* corner radius, in units of small spacing
// Keep this value in sync with Bias_Default in
// kstyle/proteametrics.h
// NOTE: this value is multiplied by settings()->smallSpacing()
// which is always 2 on wayland, but can be something else on X11
// Protea: 10 px on Wayland, the same as the Plasma Style's panels, popups
// and tooltips (scripts/frame-radius.py); a test checks they match.
static constexpr qreal Frame_FrameRadius = 5;

//* titlebar metrics, in units of small spacing
static constexpr int TitleBar_TopMargin = 2;
static constexpr int TitleBar_BottomMargin = 2;
static constexpr int TitleBar_SideMargin = 2;
static constexpr int TitleBar_ButtonSpacing = 2;

// shadow dimensions (pixels)
static constexpr int Shadow_Overlap = 3;

// frame intensities (called bias in KColorUtilities::Mix)
// Keep this value in sync with Bias_Default in
// kstyle/proteametrics.h
static constexpr qreal Bias_Default = 0.20;

// window outline color, on focused and unfocused windows alike. It's fixed,
// so change it along with the title bar colors (Header in the color scheme);
// Breeze mixes it from the title bar color instead.
static constexpr QColor Outline_Color(0x34, 0x34, 0x35);
}

//* standard pen widths
namespace PenWidth
{
/* Using 1 instead of slightly more than 1 causes symbols drawn with
 * pen strokes to look skewed. The exact amount added does not matter
 * as long as it isn't too visible.
 */
// The standard pen stroke width for symbols.
static constexpr qreal Symbol = 1.01;
}

//* exception
enum ExceptionMask {
    None = 0,
    BorderSize = 1 << 4,
};
}
