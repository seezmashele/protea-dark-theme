################# includes #################
include_directories(
  animations
  debug
)

include_directories(${CMAKE_SOURCE_DIR}/libproteacommon)
include_directories(${CMAKE_BINARY_DIR}/libproteacommon)

################# configuration #################
configure_file(config-protea.h.cmake ${CMAKE_CURRENT_BINARY_DIR}/config-protea.h )

########### next target ###############
set(protea_PART_SRCS
    animations/proteaanimation.cpp
    animations/proteaanimations.cpp
    animations/proteaanimationdata.cpp
    animations/proteabaseengine.cpp
    animations/proteabusyindicatordata.cpp
    animations/proteabusyindicatorengine.cpp
    animations/proteadialdata.cpp
    animations/proteadialengine.cpp
    animations/proteaenabledata.cpp
    animations/proteagenericdata.cpp
    animations/proteaheaderviewdata.cpp
    animations/proteaheaderviewengine.cpp
    animations/proteascrollbardata.cpp
    animations/proteascrollbarengine.cpp
    animations/proteaspinboxengine.cpp
    animations/proteaspinboxdata.cpp
    animations/proteastackedwidgetdata.cpp
    animations/proteastackedwidgetengine.cpp
    animations/proteatabbarengine.cpp
    animations/proteatabbardata.cpp
    animations/proteatoolboxengine.cpp
    animations/proteatransitiondata.cpp
    animations/proteatransitionwidget.cpp
    animations/proteawidgetstateengine.cpp
    animations/proteawidgetstatedata.cpp
    debug/proteawidgetexplorer.cpp
    proteaaddeventfilter.cpp
    proteablurhelper.cpp
    proteaframeshadow.cpp
    proteahelper.cpp
    proteamdiwindowshadow.cpp
    proteamnemonics.cpp
    proteapropertynames.cpp
    proteashadowhelper.cpp
    proteasplitterproxy.cpp
    proteastyle.cpp
    proteastyleplugin.cpp
    proteatileset.cpp
    proteawindowmanager.cpp
    proteatoolsareamanager.cpp
)

kconfig_add_kcfg_files(protea_PART_SRCS ../kdecoration/proteasettings.kcfgc)
kconfig_add_kcfg_files(protea_PART_SRCS proteastyleconfigdata.kcfgc)


add_library(protea${QT_MAJOR_VERSION} MODULE ${protea_PART_SRCS})

ecm_qt_declare_logging_category(protea${QT_MAJOR_VERSION}
    HEADER
        protea_logging.h
    IDENTIFIER
        PROTEA
    CATEGORY_NAME
        protea${QT_MAJOR_VERSION}
    DEFAULT_SEVERITY
        Warning
)

target_link_libraries(protea${QT_MAJOR_VERSION}
    Qt${QT_MAJOR_VERSION}::Core
    Qt${QT_MAJOR_VERSION}::Gui
    Qt${QT_MAJOR_VERSION}::Widgets
)

if(HAVE_QTDBUS)
    target_link_libraries(protea${QT_MAJOR_VERSION}
        Qt${QT_MAJOR_VERSION}::DBus
    )
endif()

if( PROTEA_HAVE_QTQUICK )
    target_link_libraries(protea${QT_MAJOR_VERSION} 
        Qt${QT_MAJOR_VERSION}::Quick
        KF${QT_MAJOR_VERSION}::CoreAddons
    )
endif()

target_link_libraries(protea${QT_MAJOR_VERSION}
    KF${QT_MAJOR_VERSION}::CoreAddons
    KF${QT_MAJOR_VERSION}::ConfigCore
    KF${QT_MAJOR_VERSION}::ConfigGui
    KF${QT_MAJOR_VERSION}::GuiAddons
    KF${QT_MAJOR_VERSION}::IconThemes
    KF${QT_MAJOR_VERSION}::WindowSystem
)

if(QT_MAJOR_VERSION STREQUAL "5")
    target_link_libraries(protea5 KF5::ConfigWidgets)
else()
    target_link_libraries(protea6 KF6::ColorScheme)
endif()

target_link_libraries(protea${QT_MAJOR_VERSION} proteacommon${QT_MAJOR_VERSION})

if(KF${QT_MAJOR_VERSION}FrameworkIntegration_FOUND)
    target_link_libraries(protea${QT_MAJOR_VERSION}  KF${QT_MAJOR_VERSION}::Style)
endif()


if (WIN32)
    # As stated in https://docs.microsoft.com/en-us/cpp/c-runtime-library/math-constants M_PI only gets defined
    # when if _USE_MATH_DEFINES is defined
    target_compile_definitions(protea${QT_MAJOR_VERSION}  PRIVATE _USE_MATH_DEFINES _BSD_SOURCE)
endif()


########### install files ###############
install(TARGETS protea${QT_MAJOR_VERSION}  DESTINATION ${KDE_INSTALL_QTPLUGINDIR}/styles/)
install(FILES protea.themerc  DESTINATION  ${KDE_INSTALL_DATADIR}/kstyle/themes)
install(FILES protea.kcfg DESTINATION ${KDE_INSTALL_KCFGDIR})

########### subdirectories ###############

if (QT_MAJOR_VERSION EQUAL "6" AND TARGET "KF6::KCMUtils")
    add_subdirectory(config)
endif()
