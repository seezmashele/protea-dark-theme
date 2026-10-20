set(proteadecoration_config_SRCS
    ../proteaexceptionlist.cpp
    proteaconfigwidget.cpp
    proteadetectwidget.cpp
    proteaexceptiondialog.cpp
    proteaexceptionlistwidget.cpp
    proteaexceptionmodel.cpp
    proteaitemmodel.cpp
)
ki18n_wrap_ui(proteadecoration_config_SRCS
   ui/proteaconfigurationui.ui
   ui/proteaexceptiondialog.ui
   ui/proteaexceptionlistwidget.ui
)

kcoreaddons_add_plugin(kcm_proteadecoration SOURCES kcm_proteadecoration.cpp ${proteadecoration_config_SRCS} INSTALL_NAMESPACE "${KDECORATION_KCM_PLUGIN_DIR}")
kconfig_add_kcfg_files(kcm_proteadecoration ../proteasettings.kcfgc)
target_include_directories(kcm_proteadecoration PRIVATE ${CMAKE_SOURCE_DIR}/kdecoration ${CMAKE_BINARY_DIR}/kdecoration/)
target_link_libraries(kcm_proteadecoration
    PUBLIC
        Qt6::Core
        Qt6::Gui
        Qt6::DBus
    PRIVATE
        KF6::ConfigCore
        KF6::CoreAddons
        KF6::GuiAddons
        KF6::I18n
        KF6::IconThemes
        KF6::KCMUtils
)
kcmutils_generate_desktop_file(kcm_proteadecoration)

# kcmshell6 < 6.27.0 doesn't know the --smoke-test param, so skip this test for
# now.
# TODO: Drop this when KF6_MIN_VERSION >= 6.27.0
if(KF6KCMUtils_VERSION VERSION_GREATER_EQUAL "6.27.0")
    add_test(NAME kcm_proteadecoration_smoketest COMMAND kcmshell6 -style Protea --smoke-test ${KDECORATION_KCM_PLUGIN_DIR}/kcm_proteadecoration)
    set_tests_properties(kcm_proteadecoration_smoketest PROPERTIES
        ENVIRONMENT_MODIFICATION QT_PLUGIN_PATH=path_list_prepend:${CMAKE_BINARY_DIR}/bin
    )
endif()
