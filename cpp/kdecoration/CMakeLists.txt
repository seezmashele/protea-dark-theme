add_definitions(-DTRANSLATION_DOMAIN="protea_kwin_deco")

################# includes #################
include_directories(${CMAKE_SOURCE_DIR}/libproteacommon)
include_directories(${CMAKE_BINARY_DIR}/libproteacommon)

################# newt target #################
### plugin classes
set(proteadecoration_SRCS
    proteabutton.cpp
    proteadecoration.cpp
    proteasettingsprovider.cpp
    proteaexceptionlist.cpp
)

### build library
kconfig_add_kcfg_files(proteadecoration_SRCS proteasettings.kcfgc)
add_library(proteadecoration MODULE
    ${proteadecoration_SRCS}
    ${proteadecoration_config_PART_FORMS_HEADERS})

set_target_properties(proteadecoration PROPERTIES
    OUTPUT_NAME org.proteadesktop.protea
)

target_link_libraries(proteadecoration
    PRIVATE
        proteacommon6
        Qt6::DBus
        KF6::CoreAddons
        KF6::ConfigGui
        KF6::GuiAddons
        KF6::I18n
        KF6::IconThemes
        KDecoration3::KDecoration
        KF6::ColorScheme
)

install(TARGETS proteadecoration DESTINATION ${KDE_INSTALL_PLUGINDIR}/${KDECORATION_PLUGIN_DIR})
install(FILES proteasettingsdata.kcfg DESTINATION ${KDE_INSTALL_KCFGDIR} RENAME protea-decoration.kcfg)

add_subdirectory(config)

