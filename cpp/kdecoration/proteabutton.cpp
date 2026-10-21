/*
 * SPDX-FileCopyrightText: 2014 Martin Gräßlin <mgraesslin@kde.org>
 * SPDX-FileCopyrightText: 2014 Hugo Pereira Da Costa <hugo.pereira@free.fr>
 *
 * SPDX-License-Identifier: GPL-2.0-only OR GPL-3.0-only OR LicenseRef-KDE-Accepted-GPL
 */
#include "proteabutton.h"

#include <KColorUtils>
#include <KDecoration3/DecoratedWindow>
#include <KIconLoader>

#include <QPainter>
#include <QPainterPath>
#include <QVariantAnimation>

namespace Protea
{
using KDecoration3::ColorGroup;
using KDecoration3::ColorRole;
using KDecoration3::DecorationButtonType;

//__________________________________________________________________
Button::Button(DecorationButtonType type, Decoration *decoration, QObject *parent)
    : DecorationButton(type, decoration, parent)
    , m_animation(new QVariantAnimation(this))
{
    // setup animation
    // It is important start and end value are of the same type, hence 0.0 and not just 0
    m_animation->setStartValue(0.0);
    m_animation->setEndValue(1.0);
    m_animation->setEasingCurve(QEasingCurve::InOutQuad);
    connect(m_animation, &QVariantAnimation::valueChanged, this, [this](const QVariant &value) {
        setOpacity(value.toReal());
    });

    // connections
    connect(decoration->window(), SIGNAL(iconChanged(QIcon)), this, SLOT(update()));
    connect(decoration->settings().get(), &KDecoration3::DecorationSettings::reconfigured, this, &Button::reconfigure);
    connect(this, &KDecoration3::DecorationButton::hoveredChanged, this, &Button::updateAnimationState);

    reconfigure();
}

//__________________________________________________________________
Button::Button(QObject *parent, const QVariantList &args)
    : Button(args.at(0).value<DecorationButtonType>(), args.at(1).value<Decoration *>(), parent)
{
    setGeometry(QRectF(QPointF(0, 0), preferredSize()));
}

//__________________________________________________________________
Button *Button::create(DecorationButtonType type, KDecoration3::Decoration *decoration, QObject *parent)
{
    if (auto d = qobject_cast<Decoration *>(decoration)) {
        Button *b = new Button(type, d, parent);
        const auto c = d->window();
        switch (type) {
        case DecorationButtonType::Close:
            b->setVisible(c->isCloseable());
            QObject::connect(c, &KDecoration3::DecoratedWindow::closeableChanged, b, &Protea::Button::setVisible);
            break;

        case DecorationButtonType::Maximize:
            b->setVisible(c->isMaximizeable());
            QObject::connect(c, &KDecoration3::DecoratedWindow::maximizeableChanged, b, &Protea::Button::setVisible);
            break;

        case DecorationButtonType::Minimize:
            b->setVisible(c->isMinimizeable());
            QObject::connect(c, &KDecoration3::DecoratedWindow::minimizeableChanged, b, &Protea::Button::setVisible);
            break;

        case DecorationButtonType::ContextHelp:
            b->setVisible(c->providesContextHelp());
            QObject::connect(c, &KDecoration3::DecoratedWindow::providesContextHelpChanged, b, &Protea::Button::setVisible);
            break;

        case DecorationButtonType::Shade:
            b->setVisible(c->isShadeable());
            QObject::connect(c, &KDecoration3::DecoratedWindow::shadeableChanged, b, &Protea::Button::setVisible);
            break;

        case DecorationButtonType::Menu:
            QObject::connect(c, &KDecoration3::DecoratedWindow::iconChanged, b, [b]() {
                b->update();
            });
            break;

        default:
            break;
        }

        return b;
    }

    return nullptr;
}

//__________________________________________________________________
void Button::paint(QPainter *painter, const QRectF &repaintRegion)
{
    Q_UNUSED(repaintRegion)

    if (!decoration()) {
        return;
    }

    switch (type()) {
    case KDecoration3::DecorationButtonType::Menu: {
        QRectF iconRect = geometry().marginsRemoved(m_padding);
        if (auto deco = qobject_cast<Decoration *>(decoration()); deco && deco->rectangularButtons()) {
            // Protea: the window icon keeps a round button's size, centered
            // in the rectangle
            const qreal side = std::min<qreal>(iconRect.height(), deco->buttonSize());
            QRectF square(0, 0, side, side);
            square.moveCenter(iconRect.center());
            iconRect = square;
        }
        const auto c = decoration()->window();
        if (auto deco = qobject_cast<Decoration *>(decoration())) {
            const QPalette activePalette = KIconLoader::global()->customPalette();
            QPalette palette = c->palette();
            palette.setColor(QPalette::WindowText, deco->fontColor());
            KIconLoader::global()->setCustomPalette(palette);
            c->icon().paint(painter, iconRect.toRect());
            if (activePalette == QPalette()) {
                KIconLoader::global()->resetPalette();
            } else {
                KIconLoader::global()->setCustomPalette(palette);
            }
        } else {
            c->icon().paint(painter, iconRect.toRect());
        }
        break;
    }
    case KDecoration3::DecorationButtonType::Spacer:
        break;
    default:
        painter->save();
        drawIcon(painter);
        painter->restore();
        break;
    }
}

//__________________________________________________________________
void Button::drawIcon(QPainter *painter) const
{
    painter->setRenderHints(QPainter::Antialiasing);

    /*
    scale painter so that its window matches QRect( -1, -1, 20, 20 )
    this makes all further rendering and scaling simpler
    all further rendering is performed inside QRect( 0, 0, 18, 18 )
    */
    const QRectF rect = geometry().marginsRemoved(m_padding);
    const qreal width(rect.width());

    // render background, filling the button
    const QColor backgroundColor(this->backgroundColor());
    if (backgroundColor.isValid()) {
        painter->save();
        painter->setPen(Qt::NoPen);
        painter->setBrush(backgroundColor);
        auto d = qobject_cast<Decoration *>(decoration());
        if (d && d->rectangularButtons()) {
            // Protea: a rectangle, kept inside the window's rounded corners
            // (a path, not a clip, so the curve stays antialiased)
            QPainterPath button;
            button.addRect(rect);
            painter->drawPath(button.intersected(d->titleBarShape()));
        } else {
            painter->translate(rect.topLeft());
            painter->scale(width / 20, width / 20);
            painter->translate(1, 1);
            painter->drawEllipse(QRectF(0, 0, 18, 18));
        }
        painter->restore();
    }

    // Protea: the mark keeps the size it has in a Breeze button (2 grid
    // units, or the button if it's smaller), centered, so bigger buttons
    // don't mean bigger symbols
    const qreal markWidth = std::min<qreal>(width, 2 * decoration()->settings()->gridUnit());
    painter->translate(rect.center());
    painter->scale(markWidth / 20, markWidth / 20);
    painter->translate(-9, -9);

    // render mark
    QColor foregroundColor(this->foregroundColor());
    if (foregroundColor.isValid()) {
        // Protea: the symbols are slightly see-through
        foregroundColor.setAlphaF(foregroundColor.alphaF() * Metrics::Button_SymbolOpacity);

        // setup painter
        QPen pen(foregroundColor);
        pen.setCapStyle(Qt::RoundCap);
        pen.setJoinStyle(Qt::MiterJoin);
        pen.setWidthF(PenWidth::Symbol * qMax((qreal)1.0, 20 / markWidth));

        painter->setPen(pen);
        painter->setBrush(Qt::NoBrush);

        // Protea: minimize, maximize, restore and close are Windows-style,
        // inside a 10x10 box (4..14), with lines on pixel centers so they
        // stay sharp at 100% scale
        switch (type()) {
        case DecorationButtonType::Close: {
            painter->drawLine(QPointF(4.5, 4.5), QPointF(13.5, 13.5));
            painter->drawLine(QPointF(13.5, 4.5), QPointF(4.5, 13.5));
            break;
        }

        case DecorationButtonType::Maximize: {
            if (isChecked()) {
                // restore: two squares, the back one hidden behind the front
                const QRectF front(4.5, 6.5, 7, 7);
                const QRectF back(6.5, 4.5, 7, 7);
                QPainterPath visible;
                visible.addRect(QRectF(0, 0, 18, 18));
                QPainterPath hidden;
                hidden.addRect(front.adjusted(-0.5, -0.5, 0.5, 0.5));

                painter->save();
                painter->setClipPath(visible.subtracted(hidden), Qt::IntersectClip);
                painter->drawRect(back);
                painter->restore();
                painter->drawRect(front);

            } else {
                painter->drawRect(QRectF(4.5, 4.5, 9, 9));
            }
            break;
        }

        case DecorationButtonType::Minimize: {
            painter->drawLine(QPointF(4.5, 9.5), QPointF(13.5, 9.5));
            break;
        }

        case DecorationButtonType::OnAllDesktops: {
            painter->setPen(Qt::NoPen);
            painter->setBrush(foregroundColor);

            if (isChecked()) {
                // outer ring
                painter->drawEllipse(QRectF(3, 3, 12, 12));

                // center dot
                QColor backgroundColor(this->backgroundColor());
                auto d = qobject_cast<Decoration *>(decoration());
                if (!backgroundColor.isValid() && d) {
                    backgroundColor = d->titleBarColor();
                }

                if (backgroundColor.isValid()) {
                    painter->setBrush(backgroundColor);
                    painter->drawEllipse(QRectF(8, 8, 2, 2));
                }

            } else {
                painter->drawPolygon(QVector<QPointF>{QPointF(6.5, 8.5), QPointF(12, 3), QPointF(15, 6), QPointF(9.5, 11.5)});

                painter->setPen(pen);
                painter->drawLine(QPointF(5.5, 7.5), QPointF(10.5, 12.5));
                painter->drawLine(QPointF(12, 6), QPointF(4.5, 13.5));
            }
            break;
        }

        case DecorationButtonType::Shade: {
            if (isChecked()) {
                painter->drawLine(QPointF(4, 5.5), QPointF(14, 5.5));
                painter->drawPolyline(QVector<QPointF>{QPointF(4, 8), QPointF(9, 13), QPointF(14, 8)});

            } else {
                painter->drawLine(QPointF(4, 5.5), QPointF(14, 5.5));
                painter->drawPolyline(QVector<QPointF>{QPointF(4, 13), QPointF(9, 8), QPointF(14, 13)});
            }

            break;
        }

        case DecorationButtonType::KeepBelow: {
            painter->drawPolyline(QVector<QPointF>{QPointF(4, 5), QPointF(9, 10), QPointF(14, 5)});

            painter->drawPolyline(QVector<QPointF>{QPointF(4, 9), QPointF(9, 14), QPointF(14, 9)});
            break;
        }

        case DecorationButtonType::KeepAbove: {
            painter->drawPolyline(QVector<QPointF>{QPointF(4, 9), QPointF(9, 4), QPointF(14, 9)});

            painter->drawPolyline(QVector<QPointF>{QPointF(4, 13), QPointF(9, 8), QPointF(14, 13)});
            break;
        }

        case DecorationButtonType::ExcludeFromCapture: {
            // A spy hat (like view-private.svg icon)
            const qreal cx = 9, cy = 9;
            // Hat crown with dip/crease at top (filled)
            std::array crownPoints{QPointF(cx - 4.1, cy - 2.5),
                                   QPointF(cx - 3.2, cy - 6.1),
                                   QPointF(cx, cy - 5),
                                   QPointF(cx + 3.2, cy - 6.1),
                                   QPointF(cx + 4.1, cy - 2.5)};
            painter->setBrush(pen.color());
            painter->drawPolygon(crownPoints.data(), crownPoints.size());
            painter->setBrush(Qt::NoBrush);
            // Hat brim
            painter->drawLine(QPointF(cx - 6.2, cy - 0.5), QPointF(cx + 6.2, cy - 0.5));
            // Glasses' lenses
            painter->drawEllipse(QPointF(cx - 4, cy + 3.8), 2.3, 2.3);
            painter->drawEllipse(QPointF(cx + 4, cy + 3.8), 2.3, 2.3);
            // Bridge between lenses
            painter->drawLine(QPointF(cx - 1.5, cy + 3.8), QPointF(cx + 1.5, cy + 3.8));
            break;
        }

        case DecorationButtonType::ApplicationMenu: {
            painter->drawRect(QRectF(3.5, 4.5, 11, 1));
            painter->drawRect(QRectF(3.5, 8.5, 11, 1));
            painter->drawRect(QRectF(3.5, 12.5, 11, 1));
            break;
        }

        case DecorationButtonType::ContextHelp: {
            QPainterPath path;
            path.moveTo(5, 6);
            path.arcTo(QRectF(5, 3.5, 8, 5), 180, -180);
            path.cubicTo(QPointF(12.5, 9.5), QPointF(9, 7.5), QPointF(9, 11.5));
            painter->drawPath(path);

            painter->drawRect(QRectF(9, 15, 0.5, 0.5));

            break;
        }

        default:
            break;
        }
    }
}

//__________________________________________________________________
QColor Button::foregroundColor() const
{
    auto d = qobject_cast<Decoration *>(decoration());
    if (!d) {
        return QColor();

    } else if (isPressed()) {
        // Protea: the symbol keeps its color on the translucent highlight
        return d->fontColor();

    } else if (type() == DecorationButtonType::Close && d->internalSettings()->outlineCloseButton()) {
        return d->titleBarColor();

    } else if ((type() == DecorationButtonType::KeepBelow || type() == DecorationButtonType::KeepAbove || type() == DecorationButtonType::Shade
                || type() == DecorationButtonType::ExcludeFromCapture)
               && isChecked()) {
        return d->titleBarColor();

    } else {
        // Protea: hovering doesn't change the symbol's color, only the
        // background behind it
        return d->fontColor();
    }
}

//__________________________________________________________________
QColor Button::backgroundColor() const
{
    auto d = qobject_cast<Decoration *>(decoration());
    if (!d) {
        return QColor();
    }

    auto c = d->window();
    QColor redColor(c->color(ColorGroup::Warning, ColorRole::Foreground));
    // Protea: the red is slightly see-through, like the symbols
    redColor.setAlphaF(redColor.alphaF() * Metrics::Button_CloseOpacity);

    // Protea: hovered and pressed buttons get translucent white behind them,
    // fainter on the bigger rectangular ones, and Close turns the color
    // scheme's red
    auto highlight = [](qreal opacity) {
        QColor color(Qt::white);
        color.setAlphaF(opacity);
        return color;
    };
    const bool rectangular = d->rectangularButtons();
    const qreal hoverOpacity = rectangular ? Metrics::Button_RectangleHoverOpacity : Metrics::Button_HoverOpacity;
    const qreal pressedOpacity = rectangular ? Metrics::Button_RectanglePressedOpacity : Metrics::Button_PressedOpacity;

    if (isPressed()) {
        if (type() == DecorationButtonType::Close) {
            return redColor.darker();
        } else {
            return highlight(pressedOpacity);
        }

    } else if ((type() == DecorationButtonType::KeepBelow || type() == DecorationButtonType::KeepAbove || type() == DecorationButtonType::Shade)
               && isChecked()) {
        return d->fontColor();
    } else if (type() == DecorationButtonType::ExcludeFromCapture && isChecked()) {
        return redColor;
    } else if (m_animation->state() == QAbstractAnimation::Running) {
        if (type() == DecorationButtonType::Close) {
            if (d->internalSettings()->outlineCloseButton()) {
                return c->isActive() ? KColorUtils::mix(redColor, redColor.lighter(), m_opacity) : KColorUtils::mix(redColor.lighter(), redColor, m_opacity);

            } else {
                QColor color(redColor);
                color.setAlpha(color.alpha() * m_opacity);
                return color;
            }

        } else {
            return highlight(hoverOpacity * m_opacity);
        }

    } else if (isHovered()) {
        if (type() == DecorationButtonType::Close) {
            return d->internalSettings()->outlineCloseButton() && c->isActive() ? redColor.lighter() : redColor;
        } else {
            return highlight(hoverOpacity);
        }

    } else if (type() == DecorationButtonType::Close && d->internalSettings()->outlineCloseButton()) {
        return c->isActive() ? redColor : d->fontColor();

    } else {
        return QColor();
    }
}

//________________________________________________________________
void Button::reconfigure()
{
    // animation
    auto d = qobject_cast<Decoration *>(decoration());
    if (!d) {
        return;
    }

    switch (type()) {
    case KDecoration3::DecorationButtonType::Spacer:
        setPreferredSize(QSizeF(d->buttonSize() * 0.5, d->buttonSize()));
        break;
    default:
        setPreferredSize(QSizeF(d->buttonSize(), d->buttonSize()));
        break;
    }

    m_animation->setDuration(d->animationsDuration());
}

//__________________________________________________________________
void Button::updateAnimationState(bool hovered)
{
    auto d = qobject_cast<Decoration *>(decoration());
    if (!(d && d->animationsDuration() > 0)) {
        return;
    }

    m_animation->setDirection(hovered ? QAbstractAnimation::Forward : QAbstractAnimation::Backward);
    if (m_animation->state() != QAbstractAnimation::Running) {
        m_animation->start();
    }
}

} // namespace
