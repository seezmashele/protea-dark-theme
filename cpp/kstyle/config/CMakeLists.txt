### configuration plugin
add_definitions(-DTRANSLATION_DOMAIN=\"protea_style_config\")
set(kstyle_protea_config_PART_SRCS proteastyleconfig.cpp)

set(kstyle_protea_config_PART_SRCS ${kstyle_protea_config_PART_SRCS} proteastyleconfigmodule.cpp)

kconfig_add_kcfg_files(kstyle_protea_config_PART_SRCS ../proteastyleconfigdata.kcfgc)
set(kstyle_protea_config_PART_FORMS ui/proteastyleconfig.ui)
ki18n_wrap_ui(kstyle_protea_config_PART_FORMS_HEADERS ${kstyle_protea_config_PART_FORMS})

kcoreaddons_add_plugin(proteastyleconfig SOURCES
    ${kstyle_protea_config_PART_SRCS}
    ${kstyle_protea_config_PART_FORMS_HEADERS}
    INSTALL_NAMESPACE "kstyle_config"
)

target_link_libraries(proteastyleconfig
    PUBLIC
        Qt6::Core
        Qt6::Gui
        Qt6::Widgets
)

if(HAVE_QTDBUS)
    target_link_libraries(proteastyleconfig
        PUBLIC
            Qt6::DBus
    )
endif()

target_link_libraries(proteastyleconfig
    PUBLIC
        KF6::CoreAddons
        KF6::ConfigCore
        KF6::ConfigWidgets
        KF6::I18n
)
if(QT_MAJOR_VERSION STREQUAL "6")
    target_link_libraries(proteastyleconfig
        PUBLIC
    KF6::KCMUtils
)
endif()
kcmutils_generate_desktop_file(proteastyleconfig)

set(protea_settings_SOURCES main.cpp)
add_executable(protea-settings ${protea_settings_SOURCES} )
target_link_libraries(protea-settings Qt6::Core Qt6::Gui Qt6::Widgets )
target_link_libraries(protea-settings KF6::I18n KF6::KCMUtils KF6::CoreAddons)

set_target_properties(protea-settings PROPERTIES OUTPUT_NAME protea-settings6)

install(TARGETS protea-settings ${KDE_INSTALL_TARGETS_DEFAULT_ARGS})

# icon
ecm_install_icons(ICONS sc-apps-protea-settings.svgz
    DESTINATION ${KDE_INSTALL_ICONDIR}
    THEME hicolor )

# kcmshell6 < 6.27.0 doesn't know the --smoke-test param, so skip this test for
# now.
# TODO: Drop this when KF6_MIN_VERSION >= 6.27.0
if(KF6KCMUtils_VERSION VERSION_GREATER_EQUAL "6.27.0")
    add_test(NAME proteastyleconfig_smoketest COMMAND kcmshell6 -style Protea --smoke-test kstyle_config/proteastyleconfig)
    set_tests_properties(proteastyleconfig_smoketest PROPERTIES
        ENVIRONMENT_MODIFICATION QT_PLUGIN_PATH=path_list_prepend:${CMAKE_BINARY_DIR}/bin
    )
endif()
